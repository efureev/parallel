package parallel

import (
	"context"
	"testing"
	"time"
)

func TestManager_NameReplace(t *testing.T) {
	cmd := Command{Cmd: "echo", Args: []string{"hello"}}
	if got := nameReplace(cmd); got != "echo hello" {
		t.Fatalf("unexpected nameReplace result: %q", got)
	}

	cmd = Command{Cmd: "echo", Args: []string{"hello"}, Format: Format{CmdName: "%CMD_NAME%-%CMD_ARGS%"}}
	if got := nameReplace(cmd); got != "echo-hello" {
		t.Fatalf("unexpected formatted nameReplace result: %q", got)
	}
}

func TestManager_ExecuteRespectsContextCancel(t *testing.T) {
	logger := Logger()
	ce := NewManager(logger)
	mgr, ok := ce.(*manager)
	if !ok {
		t.Fatalf("unexpected manager type: %T", ce)
	}

	cmd := Command{Cmd: "sleep", Args: []string{"5"}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := mgr.Execute(ctx, cmd)
	if err == nil {
		t.Fatalf("expected error due to context cancellation, got nil")
	}
}
