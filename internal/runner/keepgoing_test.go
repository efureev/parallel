package runner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// gatedRunner: одна команда падает, остальные ждут реакции на её отказ.
//
// Соседняя команда ждёт отмены контекста, а не какого-либо события от самого
// раннера. Это единственный наблюдаемый признак, которым режимы отличаются:
// при fail-fast отмена придёт, при keep-going — нет. Разблокировать соседа
// закрытием собственного канала нельзя: он проснулся бы раньше, чем errgroup
// успел отменить контекст, и тест мерил бы скорость планировщика.
//
// noCancelWait — сколько ждать отмены, прежде чем счесть, что её не будет.
// Ожидание платится один раз и только в тесте на keep-going: доказать
// отсутствие события иначе, чем ожиданием, нельзя.
type gatedRunner struct {
	failing      string
	err          error
	noCancelWait time.Duration

	started sync.WaitGroup
}

func newGatedRunner(failing string, err error, chains int, noCancelWait time.Duration) *gatedRunner {
	r := &gatedRunner{failing: failing, err: err, noCancelWait: noCancelWait}
	r.started.Add(chains)

	return r
}

func (g *gatedRunner) Execute(ctx context.Context, _ *flow.CommandChain, cmd flow.Command) error {
	g.started.Done()
	// Обе команды должны дойти до запуска, иначе сосед может не начаться вовсе.
	g.started.Wait()

	if cmd.Name == g.failing {
		return g.err
	}

	if g.noCancelWait == 0 {
		// Режим fail-fast: отмена гарантирована, ждём её сколько потребуется.
		<-ctx.Done()

		return ctx.Err()
	}

	timer := time.NewTimer(g.noCancelWait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (g *gatedRunner) ExecuteWithPipe(ctx context.Context, chain *flow.CommandChain, cmd flow.Command) error {
	return g.Execute(ctx, chain, cmd)
}

// twoChains собирает две цепочки по одной команде: падающую и обычную.
func twoChains() []*flow.CommandChain {
	bad := &flow.CommandChain{Name: "bad"}
	bad.Add(flow.Command{Name: "boom", Cmd: "echo"})

	good := &flow.CommandChain{Name: "good"}
	good.Add(flow.Command{Name: "fine", Cmd: "echo"})

	return []*flow.CommandChain{bad, good}
}

// TestExecuteParallel_FailFastStopsSiblings фиксирует поведение по умолчанию:
// оно не должно измениться от появления keep-going.
func TestExecuteParallel_FailFastStopsSiblings(t *testing.T) {
	// Ноль: отмена обязана прийти, ждём её без ограничения по времени.
	runner := newGatedRunner("boom", errFakeA, 2, 0)
	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	if err := exec.ExecuteParallel(t.Context(), twoChains()); !errors.Is(err, errFakeA) {
		t.Fatalf("ожидалась errFakeA, получено %v", err)
	}

	results := exec.results
	if !results[0].Failed() {
		t.Error("упавшая цепочка не помечена отказом")
	}

	if !results[1].Stopped {
		t.Error("соседнюю цепочку не остановил отказ — это и есть fail-fast")
	}
}

// TestExecuteParallel_KeepGoingRunsSiblings — ради этого режима задача и
// делалась: в CI нужны все отказы разом, а не первый и ещё два прогона.
func TestExecuteParallel_KeepGoingRunsSiblings(t *testing.T) {
	// Отмены быть не должно; ждём заведомо дольше, чем она бы шла.
	runner := newGatedRunner("boom", errFakeA, 2, 100*time.Millisecond)
	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withKeepGoing())

	if err := exec.ExecuteParallel(t.Context(), twoChains()); !errors.Is(err, errFakeA) {
		t.Fatalf("ожидалась errFakeA, получено %v", err)
	}

	results := exec.results
	if !results[0].Failed() {
		t.Error("упавшая цепочка не помечена отказом")
	}

	if results[1].Failed() {
		t.Errorf("соседняя цепочка не падала: %v", results[1].Err)
	}

	if results[1].Stopped {
		t.Error("соседнюю цепочку остановили, хотя keep-going это и запрещает")
	}
}

// TestExecuteParallel_KeepGoingObeysSignal: keep-going отключает отмену по
// отказу, но не обработку Ctrl+C — иначе утилиту стало бы нечем остановить.
func TestExecuteParallel_KeepGoingObeysSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	blocked := make(chan struct{})
	runner := &blockingRunner{entered: blocked}

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withKeepGoing())

	done := make(chan error, 1)

	go func() {
		done <- exec.ExecuteParallel(ctx, twoChains())
	}()

	<-blocked
	cancel()

	select {
	case <-done:
	case <-t.Context().Done():
		t.Fatal("keep-going не отреагировал на отмену контекста")
	}

	for _, res := range exec.results {
		if !res.Stopped {
			t.Errorf("цепочка %q не помечена остановленной по сигналу", res.Name)
		}
	}
}

// blockingRunner держит команду до отмены контекста.
type blockingRunner struct {
	entered chan struct{}
	once    sync.Once
}

func (b *blockingRunner) Execute(ctx context.Context, _ *flow.CommandChain, _ flow.Command) error {
	b.once.Do(func() { close(b.entered) })

	<-ctx.Done()

	return ctx.Err()
}

func (b *blockingRunner) ExecuteWithPipe(ctx context.Context, chain *flow.CommandChain, cmd flow.Command) error {
	return b.Execute(ctx, chain, cmd)
}

// TestExitCode_DeterministicWithKeepGoing — побочный выигрыш режима: при
// fail-fast отказ соседа маскируется отменой, и код возврата на одном и том же
// файле пляшет. При keep-going доезжают все отказы, и «первый в порядке
// объявления» становится честным правилом.
func TestExitCode_DeterministicWithKeepGoing(t *testing.T) {
	const runs = 20

	for range runs {
		barrier := &sync.WaitGroup{}
		barrier.Add(2)

		runner := &failingRunner{
			byName: map[string]error{
				"first":  &ExitError{Chain: "a", Command: "first", Code: 7},
				"second": &ExitError{Chain: "b", Command: "second", Code: 42},
			},
			barrier: barrier,
		}

		exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withKeepGoing())

		chainA := &flow.CommandChain{Name: "a"}
		chainA.Add(flow.Command{Name: "first", Cmd: "echo"})

		chainB := &flow.CommandChain{Name: "b"}
		chainB.Add(flow.Command{Name: "second", Cmd: "echo"})

		err := exec.ExecuteParallel(t.Context(), []*flow.CommandChain{chainA, chainB})

		if code := ExitCode(err, 1); code != 7 {
			t.Fatalf("код возврата = %d, ожидался 7 (первая цепочка по порядку объявления)", code)
		}
	}
}

// TestManager_WithKeepGoing сторожит проводку опции: она применяется до сборки
// chainExecutor, и молча потеряться здесь легко.
func TestManager_WithKeepGoing(t *testing.T) {
	if mgr := NewManager(ui.NewDiscardLogger(), nil); mgr.chains.keepGoing {
		t.Error("keep-going включён без опции")
	}

	if mgr := NewManager(ui.NewDiscardLogger(), nil, WithKeepGoing()); !mgr.chains.keepGoing {
		t.Error("опция WithKeepGoing не доехала до chainExecutor")
	}
}
