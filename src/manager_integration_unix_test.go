//go:build !windows

package parallel

import (
	"context"
	"errors"
	"testing"
	"time"
)

// newTestManager returns a concrete *manager for integration tests.
func newTestManager(t *testing.T) *manager {
	t.Helper()

	ce := NewManager(Logger())

	mgr, ok := ce.(*manager)
	if !ok {
		t.Fatalf("unexpected manager type: %T", ce)
	}

	return mgr
}

// shCommand builds a Command running the given shell script, attached to a chain.
func shCommand(name, script string, pipe bool) Command {
	chain := &CommandChain{Name: "chain-" + name}
	cmd := Command{Name: name, Cmd: "sh", Args: []string{"-c", script}, Pipe: pipe}
	chain.Add(cmd)

	return chain.commands[0]
}

func TestManager_ExecuteSuccess(t *testing.T) {
	mgr := newTestManager(t)

	cmd := shCommand("ok", "echo hello; echo oops 1>&2", false)

	if err := mgr.Execute(t.Context(), cmd); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestManager_ExecuteFailureExitCode(t *testing.T) {
	mgr := newTestManager(t)

	cmd := shCommand("fail", "exit 3", false)

	err := mgr.Execute(t.Context(), cmd)
	if err == nil {
		t.Fatalf("expected error for non-zero exit, got nil")
	}

	if !errors.Is(err, ErrCommandExecution) {
		t.Fatalf("expected ErrCommandExecution, got %v", err)
	}
}

func TestManager_ExecuteWithPipeSuccess(t *testing.T) {
	mgr := newTestManager(t)

	cmd := shCommand("pipe-ok", "echo line1; echo line2; echo err 1>&2", true)

	if err := mgr.ExecuteWithPipe(t.Context(), cmd); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestManager_ExecuteWithPipeFailureExitCode(t *testing.T) {
	mgr := newTestManager(t)

	cmd := shCommand("pipe-fail", "echo before; exit 2", true)

	err := mgr.ExecuteWithPipe(t.Context(), cmd)
	if err == nil {
		t.Fatalf("expected error for non-zero exit, got nil")
	}

	if !errors.Is(err, ErrCommandExecution) {
		t.Fatalf("expected ErrCommandExecution, got %v", err)
	}
}

func TestManager_ExecuteWithPipeContextCancel(t *testing.T) {
	mgr := newTestManager(t)

	cmd := shCommand("pipe-cancel", "sleep 30", true)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	err := mgr.ExecuteWithPipe(ctx, cmd)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context error, got %v", err)
	}
}

// TestManager_ExecuteForceKill ensures that a process ignoring the shutdown
// signal is force-killed after forceKillTimeout once the context is canceled.
func TestManager_ExecuteForceKill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping force-kill timing test in -short mode")
	}

	mgr := newTestManager(t)

	// Ignore TERM/INT and keep running; only SIGKILL (force) can stop it.
	cmd := shCommand("stubborn", "trap '' TERM INT; sleep 30", false)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)

	go func() {
		done <- mgr.Execute(ctx, cmd)
	}()

	// Give the process time to start and install the trap, then cancel.
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled after force kill, got %v", err)
		}
	case <-time.After(forceKillTimeout + 5*time.Second):
		t.Fatalf("process was not force-killed within the expected window")
	}
}

func TestManager_ExecuteParallelEndToEnd(t *testing.T) {
	mgr := newTestManager(t)

	data := ConfigData{
		Chains: []ChainConfig{
			{
				Name: "build",
				Commands: []NamedCommand{
					{Name: "step1", Spec: command{Cmd: []string{"sh", "-c", "echo step1"}}},
					{Name: "step2", Spec: command{Cmd: []string{"sh", "-c", "echo step2"}}},
				},
			},
			{
				Name: "serve",
				Commands: []NamedCommand{
					{Name: "srv", Spec: command{Cmd: []string{"sh", "-c", "echo serving"}, Pipe: true}},
				},
			},
		},
	}

	flow := NewFlowBuilder(Logger()).Build(data)

	if err := flow.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := mgr.ExecuteParallel(ctx, flow.Chains); err != nil {
		t.Fatalf("ExecuteParallel returned error: %v", err)
	}
}

func TestManager_ExecuteParallelPropagatesFailure(t *testing.T) {
	mgr := newTestManager(t)

	data := ConfigData{
		Chains: []ChainConfig{
			{
				Name: "boom",
				Commands: []NamedCommand{
					{Name: "fail", Spec: command{Cmd: []string{"sh", "-c", "exit 1"}}},
				},
			},
		},
	}

	flow := NewFlowBuilder(Logger()).Build(data)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := mgr.ExecuteParallel(ctx, flow.Chains); err == nil {
		t.Fatalf("expected ExecuteParallel to return failure, got nil")
	}
}
