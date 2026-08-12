//go:build !windows

package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
)

// shCommand собирает команду, выполняющую скрипт через sh, вместе с её цепочкой.
func shCommand(name, script string, pipe bool) (*flow.CommandChain, flow.Command) {
	chain := &flow.CommandChain{Name: "chain-" + name}
	chain.Add(flow.Command{Name: name, Cmd: "sh", Args: []string{"-c", script}, Pipe: pipe})

	return chain, chain.Commands()[0]
}

func TestManager_ExecuteSuccess(t *testing.T) {
	mgr := newTestManager(t)

	chain, cmd := shCommand("ok", "echo hello; echo oops 1>&2", false)

	if err := mgr.Execute(t.Context(), chain, cmd); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestManager_ExecuteFailureExitCode(t *testing.T) {
	mgr := newTestManager(t)

	chain, cmd := shCommand("fail", "exit 3", false)

	err := mgr.Execute(t.Context(), chain, cmd)
	if err == nil {
		t.Fatalf("expected error for non-zero exit, got nil")
	}

	if !errors.Is(err, ErrCommandExecution) {
		t.Fatalf("expected ErrCommandExecution, got %v", err)
	}
}

func TestManager_ExecuteWithPipeSuccess(t *testing.T) {
	mgr := newTestManager(t)

	chain, cmd := shCommand("pipe-ok", "echo line1; echo line2; echo err 1>&2", true)

	if err := mgr.ExecuteWithPipe(t.Context(), chain, cmd); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestManager_ExecuteWithPipeFailureExitCode(t *testing.T) {
	mgr := newTestManager(t)

	chain, cmd := shCommand("pipe-fail", "echo before; exit 2", true)

	err := mgr.ExecuteWithPipe(t.Context(), chain, cmd)
	if err == nil {
		t.Fatalf("expected error for non-zero exit, got nil")
	}

	if !errors.Is(err, ErrCommandExecution) {
		t.Fatalf("expected ErrCommandExecution, got %v", err)
	}
}

func TestManager_ExecuteWithPipeContextCancel(t *testing.T) {
	mgr := newTestManager(t)

	chain, cmd := shCommand("pipe-cancel", "sleep 30", true)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	err := mgr.ExecuteWithPipe(ctx, chain, cmd)
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
	chain, cmd := shCommand("stubborn", "trap '' TERM INT; sleep 30", false)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)

	go func() {
		done <- mgr.Execute(ctx, chain, cmd)
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

// buildFlow собирает Flow из описаний цепочек: имя цепочки → список sh-скриптов.
func buildFlow(chains []struct {
	name    string
	scripts []string
	pipe    bool
},
) *flow.Flow {
	result := &flow.Flow{}

	for _, c := range chains {
		chain := &flow.CommandChain{Name: c.name}

		for i, script := range c.scripts {
			chain.Add(flow.Command{
				Name: c.name + "-" + string(rune('a'+i)),
				Cmd:  "sh",
				Args: []string{"-c", script},
				Pipe: c.pipe,
			})
		}

		result.AddChain(chain)
	}

	return result
}

func TestManager_ExecuteParallelEndToEnd(t *testing.T) {
	mgr := newTestManager(t)

	result := buildFlow([]struct {
		name    string
		scripts []string
		pipe    bool
	}{
		{name: "build", scripts: []string{"echo step1", "echo step2"}, pipe: false},
		{name: "serve", scripts: []string{"echo serving"}, pipe: true},
	})

	if err := result.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := mgr.ExecuteParallel(ctx, result.Chains); err != nil {
		t.Fatalf("ExecuteParallel returned error: %v", err)
	}
}

func TestManager_ExecuteParallelPropagatesFailure(t *testing.T) {
	mgr := newTestManager(t)

	result := buildFlow([]struct {
		name    string
		scripts []string
		pipe    bool
	}{
		{name: "boom", scripts: []string{"exit 1"}, pipe: false},
	})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := mgr.ExecuteParallel(ctx, result.Chains); err == nil {
		t.Fatalf("expected ExecuteParallel to return failure, got nil")
	}
}
