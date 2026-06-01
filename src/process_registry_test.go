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
	name, args := sleepCmdNameArgs(5)
	cmd := exec.Command(name, args...)
	configureProcessGroup(cmd)
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

	// Владельцем cmd.Wait() выступает отдельная горутина (как в executor):
	// она единственная вызывает Wait и закрывает done после завершения.
	waitDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waitDone)
	}()

	reg.add(key, cmd, waitDone)

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

	// command must be finished: waitDone must be closed by now (single Wait owner).
	select {
	case <-waitDone:
		// ok
	case <-time.After(forceKillTimeout):
		t.Fatalf("command did not finish after stopAll")
	}

	// remove and ensure empty
	reg.remove(key)
	if len(reg.snapshot()) != 0 {
		t.Fatalf("expected 0 processes in registry after remove, got %d", len(reg.snapshot()))
	}
}
