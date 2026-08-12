package runner

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// testTimeouts — тайминги для тестов: миллисекунды вместо секунд.
// Раньше эти значения были константами, и один тест ждал три секунды
// из пяти с половиной на весь прогон.
var testTimeouts = Timeouts{
	ForceKill: 150 * time.Millisecond,
	Drain:     150 * time.Millisecond,
}

// Интеграционные тесты форкают настоящие процессы: только так проверяются
// супервизия, группы процессов и доставка сигналов. Они выделены в отдельную
// группу и пропускаются в -short, чтобы быстрый цикл обратной связи оставался
// быстрым.

// requireIntegration пропускает тест в коротком режиме.
func requireIntegration(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("интеграционный тест форкает реальные процессы")
	}
}

// newTestManager возвращает конкретный *Manager для тестов.
func newTestManager(t *testing.T) *Manager {
	t.Helper()

	out := ui.NewDiscardOutput()
	lgr, formatter := out.Logger(), out.Formatter()

	return NewManager(lgr, formatter, WithTimeouts(testTimeouts))
}

func TestManager_SetShutdownSignal(t *testing.T) {
	mgr := newTestManager(t)

	mgr.SetShutdownSignal(syscall.SIGINT)

	if got := mgr.getShutdownSignal(); got != os.Signal(syscall.SIGINT) {
		t.Fatalf("expected SIGINT, got %v", got)
	}
}

func TestManager_UniqueCmdKey(t *testing.T) {
	cmd := flow.Command{Name: "worker", Cmd: "echo"}

	if got := uniqueCmdKey("build", cmd, 42); got != "build/worker_42" {
		t.Errorf("with chain: got %q", got)
	}

	if got := uniqueCmdKey("", cmd, 42); got != "worker_42" {
		t.Errorf("without chain: got %q", got)
	}
}

func TestManager_ChainName(t *testing.T) {
	if got := chainName(nil); got != "" {
		t.Errorf("nil chain: got %q", got)
	}

	if got := chainName(&flow.CommandChain{Name: "c"}); got != "c" {
		t.Errorf("named chain: got %q", got)
	}
}

func TestManager_ExecuteRespectsContextCancel(t *testing.T) {
	mgr := newTestManager(t)

	name, args := sleepCmdNameArgs(5)
	cmd := flow.Command{Cmd: name, Args: args}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if err := mgr.Execute(ctx, nil, cmd); err == nil {
		t.Fatalf("expected error due to context cancellation, got nil")
	}
}
