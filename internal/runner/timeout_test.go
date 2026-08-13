package runner

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// hangingCommand собирает долгую команду вместе с её цепочкой.
//
// sleepCmdNameArgs — кроссплатформенный хелпер: на Windows утилиты sleep нет,
// и он разворачивается в ping.
func hangingCommand(t *testing.T, limit time.Duration) (*flow.CommandChain, flow.Command) {
	t.Helper()

	name, args := sleepCmdNameArgs(5)

	chain := &flow.CommandChain{Name: "slow"}
	chain.Add(flow.Command{Name: "hangs", Cmd: name, Args: args, Timeout: limit})

	return chain, chain.Commands()[0]
}

// TestManager_CommandTimeout — зависшая команда должна сниматься сама.
//
// Раньше она держала весь запуск до Ctrl+C, а в CI это худший исход: вместо
// отчёта приходит job, отвалившийся по общему лимиту, и по логам не видно,
// какая именно команда встала.
func TestManager_CommandTimeout(t *testing.T) {
	requireIntegration(t)

	mgr := newTestManager(t)
	chain, cmd := hangingCommand(t, 100*time.Millisecond)

	start := time.Now()
	err := mgr.Execute(t.Context(), chain, cmd)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("ожидалась ErrCommandTimeout, получено %v", err)
	}

	// Команда спит пять секунд: уложиться в заметно меньшее время можно только
	// если её действительно сняли, а не дождались.
	if elapsed > 3*time.Second {
		t.Errorf("снятие заняло %s — похоже, ждали завершения команды", elapsed)
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("ошибка не разворачивается в *TimeoutError: %v", err)
	}

	if timeoutErr.Chain != "slow" || timeoutErr.Command != "hangs" {
		t.Errorf("контекст потерян: chain=%q command=%q", timeoutErr.Chain, timeoutErr.Command)
	}

	if timeoutErr.Limit != 100*time.Millisecond {
		t.Errorf("предел в ошибке = %s, ожидалось 100ms", timeoutErr.Limit)
	}
}

// TestManager_TimeoutWithPipe: у потоковой формы свой путь остановки, и он
// обязан приводить к тому же результату.
func TestManager_TimeoutWithPipe(t *testing.T) {
	requireIntegration(t)

	mgr := newTestManager(t)
	chain, cmd := hangingCommand(t, 100*time.Millisecond)
	cmd.Pipe = true

	if err := mgr.ExecuteWithPipe(t.Context(), chain, cmd); !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("ожидалась ErrCommandTimeout, получено %v", err)
	}
}

// TestManager_TimeoutNotTriggeredWhenFast — обратная сторона: быстрая команда
// с большим пределом не должна пострадать.
func TestManager_TimeoutNotTriggeredWhenFast(t *testing.T) {
	requireIntegration(t)

	mgr := newTestManager(t)

	chain := &flow.CommandChain{Name: "fast"}
	chain.Add(flow.Command{Name: "echo", Cmd: "echo", Args: []string{"ok"}, Timeout: 30 * time.Second})

	if err := mgr.Execute(t.Context(), chain, chain.Commands()[0]); err != nil {
		t.Fatalf("быстрая команда снята по таймауту: %v", err)
	}
}

// TestManager_CommandTimeoutPrecedence — поле команды сильнее глобального
// флага: общий предел это страховка, а точечный — осознанное решение.
func TestManager_CommandTimeoutPrecedence(t *testing.T) {
	const (
		global = 5 * time.Second
		own    = time.Second
	)

	tests := []struct {
		name   string
		global time.Duration
		own    time.Duration
		want   time.Duration
	}{
		{name: "нет ни того ни другого", want: 0},
		{name: "только глобальный", global: global, want: global},
		{name: "только собственный", own: own, want: own},
		{name: "собственный перекрывает глобальный", global: global, own: own, want: own},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := ui.NewDiscardOutput()

			var opts []Option
			if tt.global > 0 {
				opts = append(opts, WithCommandTimeout(tt.global))
			}

			mgr := NewManager(out.Logger(), out.Formatter(), opts...)

			if got := mgr.commandTimeout(flow.Command{Timeout: tt.own}); got != tt.want {
				t.Errorf("предел = %s, ожидался %s", got, tt.want)
			}
		})
	}
}

// TestExitCode_Timeout — 124 выбран по соглашению timeout(1) из GNU coreutils:
// у снятого процесса собственного кода нет, и скрипту нужно чем-то отличать
// «не уложилась» от «упала».
func TestExitCode_Timeout(t *testing.T) {
	timedOut := &TimeoutError{Chain: "a", Command: "slow", Limit: time.Second}
	failed := &ExitError{Chain: "b", Command: "bad", Code: 42}

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "только таймаут", err: timedOut, want: timeoutExitCode},
		{name: "обёрнутый таймаут", err: errors.Join(timedOut), want: timeoutExitCode},
		{name: "таймаут первым по порядку", err: errors.Join(timedOut, failed), want: timeoutExitCode},
		{name: "отказ первым по порядку", err: errors.Join(failed, timedOut), want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err, 1); got != tt.want {
				t.Errorf("ExitCode = %d, ожидалось %d", got, tt.want)
			}
		})
	}
}

// TestTimeoutError_Message: сообщение обязано называть команду, цепочку и сам
// предел — иначе непонятно, что чинить.
func TestTimeoutError_Message(t *testing.T) {
	err := &TimeoutError{Chain: "build", Command: "compile", Limit: 90 * time.Second}

	for _, want := range []string{"build", "compile", "1m30s"} {
		if got := err.Error(); !strings.Contains(got, want) {
			t.Errorf("сообщение %q не содержит %q", got, want)
		}
	}
}
