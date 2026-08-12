package runner

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// newTestManager возвращает конкретный *manager для тестов.
func newTestManager(t *testing.T) *manager {
	t.Helper()

	ce := NewManager(ui.Logger())

	mgr, ok := ce.(*manager)
	if !ok {
		t.Fatalf("unexpected manager type: %T", ce)
	}

	return mgr
}

func TestManager_SetShutdownSignal(t *testing.T) {
	mgr := newTestManager(t)

	mgr.SetShutdownSignal(syscall.SIGINT)

	if got := mgr.getShutdownSignal(); got != syscall.SIGINT {
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

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	if err := mgr.Execute(ctx, nil, cmd); err == nil {
		t.Fatalf("expected error due to context cancellation, got nil")
	}
}
