package runner

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// scriptedRunner отдаёт заранее заданную последовательность исходов и считает
// вызовы. Настоящие процессы для проверки политики не нужны: она решает только,
// звать ли запуск ещё раз.
type scriptedRunner struct {
	outcomes []error
	calls    atomic.Int64
}

func (s *scriptedRunner) next() error {
	n := int(s.calls.Add(1)) - 1
	if n < len(s.outcomes) {
		return s.outcomes[n]
	}

	// Дальше исход повторяет последний заданный.
	if len(s.outcomes) == 0 {
		return nil
	}

	return s.outcomes[len(s.outcomes)-1]
}

func (s *scriptedRunner) Execute(_ context.Context, _ *flow.CommandChain, _ flow.Command) error {
	return s.next()
}

func (s *scriptedRunner) ExecuteWithPipe(ctx context.Context, chain *flow.CommandChain, cmd flow.Command) error {
	return s.Execute(ctx, chain, cmd)
}

// restartCmd собирает команду с заданной политикой и крохотной задержкой:
// проверяется поведение, а не тайминги.
func restartCmd(policy flow.RestartPolicy, attempts int) (*flow.CommandChain, flow.Command) {
	chain := &flow.CommandChain{Name: "svc"}
	chain.Add(flow.Command{
		Name:            "app",
		Cmd:             "echo",
		Restart:         policy,
		RestartAttempts: attempts,
		RestartDelay:    time.Millisecond,
	})

	return chain, chain.Commands()[0]
}

func runRestart(t *testing.T, runner CommandRunner, chain *flow.CommandChain, cmd flow.Command) error {
	t.Helper()

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	return exec.runWithRestart(t.Context(), chain, cmd, func(ctx context.Context) error {
		return runner.Execute(ctx, chain, cmd)
	})
}

// TestRunWithRestart_OnFailureUntilSuccess — ровно тот случай, ради которого
// задача делалась: dev-сервер падает на ошибке компиляции и поднимается сам,
// когда её починили.
func TestRunWithRestart_OnFailureUntilSuccess(t *testing.T) {
	runner := &scriptedRunner{outcomes: []error{errFakeA, errFakeA, nil}}
	chain, cmd := restartCmd(flow.RestartOnFailure, 0)

	if err := runRestart(t, runner, chain, cmd); err != nil {
		t.Fatalf("успешный третий запуск не должен давать ошибку: %v", err)
	}

	if got := runner.calls.Load(); got != 3 {
		t.Errorf("запусков = %d, ожидалось 3", got)
	}
}

// TestRunWithRestart_OnFailureIgnoresSuccess: успешную команду перезапускать
// нельзя, иначе не-pipe цепочка никогда не дойдёт до следующей команды.
func TestRunWithRestart_OnFailureIgnoresSuccess(t *testing.T) {
	runner := &scriptedRunner{outcomes: []error{nil}}
	chain, cmd := restartCmd(flow.RestartOnFailure, 0)

	if err := runRestart(t, runner, chain, cmd); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if got := runner.calls.Load(); got != 1 {
		t.Errorf("запусков = %d, ожидался 1", got)
	}
}

// TestRunWithRestart_NeverIsDefault — отсутствие поля обязано означать прежнее
// поведение: один запуск и всё.
func TestRunWithRestart_NeverIsDefault(t *testing.T) {
	runner := &scriptedRunner{outcomes: []error{errFakeA}}

	chain := &flow.CommandChain{Name: "svc"}
	chain.Add(flow.Command{Name: "app", Cmd: "echo"})

	if err := runRestart(t, runner, chain, chain.Commands()[0]); !errors.Is(err, errFakeA) {
		t.Fatalf("ожидалась errFakeA, получено %v", err)
	}

	if got := runner.calls.Load(); got != 1 {
		t.Errorf("запусков = %d, ожидался 1 — политики по умолчанию быть не должно", got)
	}
}

// TestRunWithRestart_AlwaysRestartsSuccess — в этом всё отличие always
// от on-failure.
func TestRunWithRestart_AlwaysRestartsSuccess(t *testing.T) {
	runner := &scriptedRunner{outcomes: []error{nil}}
	chain, cmd := restartCmd(flow.RestartAlways, 3)

	if err := runRestart(t, runner, chain, cmd); err != nil {
		t.Fatalf("исчерпание попыток на успехах не должно давать ошибку: %v", err)
	}

	if got := runner.calls.Load(); got != 3 {
		t.Errorf("запусков = %d, ожидалось 3", got)
	}
}

// TestRunWithRestart_AttemptsExhausted — предел соблюдается, а код возврата
// упавшей команды переживает обёртку: иначе скрипт получил бы 1 вместо 42.
func TestRunWithRestart_AttemptsExhausted(t *testing.T) {
	exitErr := &ExitError{Chain: "svc", Command: "app", Code: 42}
	runner := &scriptedRunner{outcomes: []error{exitErr}}
	chain, cmd := restartCmd(flow.RestartOnFailure, 3)

	err := runRestart(t, runner, chain, cmd)
	if err == nil {
		t.Fatal("исчерпание попыток обязано быть отказом")
	}

	if got := runner.calls.Load(); got != 3 {
		t.Errorf("запусков = %d, ожидалось 3", got)
	}

	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("в сообщении нет числа попыток: %v", err)
	}

	if code := ExitCode(err, 1); code != 42 {
		t.Errorf("код возврата = %d, ожидался 42 — обёртка потеряла ExitError", code)
	}
}

// TestRunWithRestart_StopsOnCancel: после отмены перезапускать нечего и незачем.
// Без этой проверки команда поднималась бы заново быстрее, чем её успевают
// снять, и остановить запуск стало бы невозможно.
func TestRunWithRestart_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	runner := &scriptedRunner{outcomes: []error{errFakeA}}

	chain := &flow.CommandChain{Name: "svc"}
	chain.Add(flow.Command{
		Name: "app", Cmd: "echo",
		Restart: flow.RestartAlways, RestartDelay: time.Hour,
	})

	cmd := chain.Commands()[0]
	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = exec.runWithRestart(ctx, chain, cmd, func(runCtx context.Context) error {
			return runner.Execute(runCtx, chain, cmd)
		})
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("отменённый перезапуск не вернулся — час задержки не был прерван")
	}

	if got := runner.calls.Load(); got != 1 {
		t.Errorf("запусков после отмены = %d, ожидался 1", got)
	}
}

// TestRunWithRestart_BackoffGrows — защита от busy-loop.
//
// Мгновенно падающая команда с политикой always без растущей задержки дала бы
// миллионы запусков в минуту. Проверяется нижняя граница суммарного времени:
// при задержке 20 мс три запуска не могут уложиться быстрее 20+40 мс.
func TestRunWithRestart_BackoffGrows(t *testing.T) {
	const base = 20 * time.Millisecond

	runner := &scriptedRunner{outcomes: []error{errFakeA}}

	chain := &flow.CommandChain{Name: "svc"}
	chain.Add(flow.Command{
		Name: "app", Cmd: "echo",
		Restart: flow.RestartOnFailure, RestartAttempts: 3, RestartDelay: base,
	})

	cmd := chain.Commands()[0]

	start := time.Now()
	_ = runRestart(t, runner, chain, cmd)
	elapsed := time.Since(start)

	if want := base + 2*base; elapsed < want {
		t.Errorf("три запуска заняли %s, а рост задержки требует не меньше %s", elapsed, want)
	}
}

// TestRunWithRestart_DelayCapped: потолок нужен, чтобы задержка не выросла
// до часов на долгоживущем запуске.
func TestRunWithRestart_DelayCapped(t *testing.T) {
	delay := maxRestartDelay

	for range 5 {
		delay = min(delay*restartBackoffFactor, maxRestartDelay)
	}

	if delay != maxRestartDelay {
		t.Errorf("задержка выросла выше потолка: %s", delay)
	}
}
