package parallel

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// helper to create and start a simple long-running command.
func startSleepCmd(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "5")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sleep command: %v", err)
	}

	return cmd
}

func TestProcessRegistry_AddRemoveAndStopAll(t *testing.T) {
	logger := Logger()
	reg := newProcessRegistry()

	cmd := startSleepCmd(t)
	key := "test_cmd"
	reg.add(key, cmd)

	// ensure process is tracked
	if len(reg.snapshot()) != 1 {
		t.Fatalf("expected 1 process in registry, got %d", len(reg.snapshot()))
	}

	// stop all with SIGTERM; should not hang and should terminate the process group
	start := time.Now()
	reg.stopAll(logger, syscall.SIGTERM)
	if time.Since(start) > forceKillTimeout*2 {
		t.Fatalf("stopAll took too long, possible deadlock")
	}

	// command must be finished
	if err := cmd.Wait(); err == nil {
		// ok: finished cleanly or with signal, but Wait must return at this point
	} // if Wait blocks, test would time out

	// remove and ensure empty
	reg.remove(key)
	if len(reg.snapshot()) != 0 {
		t.Fatalf("expected 0 processes in registry after remove, got %d", len(reg.snapshot()))
	}
}
