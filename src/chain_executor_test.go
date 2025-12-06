package parallel

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRunner struct {
	count int32
}

func (f *fakeRunner) Execute(ctx context.Context, command Command) error {
	atomic.AddInt32(&f.count, 1)

	return nil
}

func (f *fakeRunner) ExecuteWithPipe(ctx context.Context, command Command) error {
	atomic.AddInt32(&f.count, 1)

	return nil
}

func TestChainExecutor_ExecuteParallel(t *testing.T) {
	lgr := Logger()
	runner := &fakeRunner{}

	stopped := int32(0)
	stopAll := func() {
		atomic.StoreInt32(&stopped, 1)
	}

	exec := newChainExecutor(lgr, runner, stopAll)

	chain1 := CommandChain{Name: "c1"}
	chain1.Add(Command{Cmd: "echo"})
	chain2 := CommandChain{Name: "c2"}
	chain2.Add(Command{Cmd: "echo", Pipe: true})

	ctx := context.Background()
	if err := exec.ExecuteParallel(ctx, []CommandChain{chain1, chain2}); err != nil {
		t.Fatalf("ExecuteParallel returned error: %v", err)
	}

	if c := atomic.LoadInt32(&runner.count); c != 2 {
		t.Fatalf("expected 2 executed commands, got %d", c)
	}
}

func TestChainExecutor_CancelContextStopsExecution(t *testing.T) {
	lgr := Logger()
	runner := &fakeRunner{}
	stopAllCalled := int32(0)
	stopAll := func() {
		atomic.StoreInt32(&stopAllCalled, 1)
	}

	exec := newChainExecutor(lgr, runner, stopAll)

	chain := CommandChain{Name: "c1"}
	chain.Add(Command{Cmd: "echo"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// ExecuteParallel при немедленной отмене контекста может вернуть nil,
	// важнее, что не запускаются команды. Вызов stopAll происходит асинхронно
	// и не гарантированно завершится к моменту возврата, поэтому его не проверяем жёстко.
	_ = exec.ExecuteParallel(ctx, []CommandChain{chain})

	if c := atomic.LoadInt32(&runner.count); c != 0 {
		t.Fatalf("expected 0 executed commands on canceled context, got %d", c)
	}
}

func TestChainExecutor_SkipsDisabledCommands(t *testing.T) {
	lgr := Logger()
	runner := &fakeRunner{}

	exec := newChainExecutor(lgr, runner, nil)

	chain := CommandChain{Name: "c1"}
	chain.Add(Command{Name: "will-skip", Cmd: "echo", Disable: true})
	chain.Add(Command{Name: "will-run", Cmd: "echo"})

	ctx := context.Background()
	if err := exec.ExecuteParallel(ctx, []CommandChain{chain}); err != nil {
		t.Fatalf("ExecuteParallel returned error: %v", err)
	}

	if c := atomic.LoadInt32(&runner.count); c != 1 {
		t.Fatalf("expected only 1 executed command (disabled skipped), got %d", c)
	}
}

// fakeRunnerConcurrent allows controlling blocking behavior of piped executions.
type fakeRunnerConcurrent struct {
	pipedStarted  int32
	normalStarted int32

	// ExecuteWithPipe blocks until this channel is closed (if provided)
	pipeBlockCh chan struct{}
	// Execute signals on this channel when called (if provided)
	normalSignalCh chan struct{}

	// Optional per-command blocking channels for piped commands
	blockByName map[string]chan struct{}
}

func (f *fakeRunnerConcurrent) Execute(ctx context.Context, command Command) error {
	atomic.AddInt32(&f.normalStarted, 1)
	if f.normalSignalCh != nil {
		select {
		case f.normalSignalCh <- struct{}{}:
		default:
		}
	}

	return nil
}

func (f *fakeRunnerConcurrent) ExecuteWithPipe(ctx context.Context, command Command) error {
	atomic.AddInt32(&f.pipedStarted, 1)
	// choose specific channel by name if provided, else default common one
	ch := f.pipeBlockCh
	if f.blockByName != nil {
		if c, ok := f.blockByName[command.Name]; ok {
			ch = c
		}
	}
	if ch != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}

	return nil
}

func TestChainExecutor_PipeConcurrentWithinChain(t *testing.T) {
	lgr := Logger()

	pipeBlock := make(chan struct{})
	normalSig := make(chan struct{}, 1)

	runner := &fakeRunnerConcurrent{pipeBlockCh: pipeBlock, normalSignalCh: normalSig}
	exec := newChainExecutor(lgr, runner, nil)

	chain := CommandChain{Name: "c1"}
	chain.Add(Command{Name: "piped", Cmd: "echo", Pipe: true})
	chain.Add(Command{Name: "normal", Cmd: "echo"})

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- exec.ExecuteParallel(ctx, []CommandChain{chain})
	}()

	// We expect the non-piped command to start even while the piped one is blocked
	select {
	case <-normalSig:
		// ok, normal started while piped is still blocked
	case <-done:
		t.Fatalf("execution finished before non-piped started; expected to wait for piped")
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for sequential command to start")
	}

	// Ensure execution is still blocked (waiting for piped)
	select {
	case <-done:
		t.Fatalf("chain finished before releasing piped command")
	default:
	}

	close(pipeBlock) // release piped goroutine

	if err := <-done; err != nil {
		t.Fatalf("ExecuteParallel returned error: %v", err)
	}

	if atomic.LoadInt32(&runner.pipedStarted) != 1 || atomic.LoadInt32(&runner.normalStarted) != 1 {
		t.Fatalf("expected both commands to start, got piped=%d normal=%d", runner.pipedStarted, runner.normalStarted)
	}
}

func TestChainExecutor_WaitsForAllPiped(t *testing.T) {
	lgr := Logger()

	pipeBlock1 := make(chan struct{})
	pipeBlock2 := make(chan struct{})

	// Runner will block per command name
	runner := &fakeRunnerConcurrent{blockByName: map[string]chan struct{}{"p1": pipeBlock1, "p2": pipeBlock2}}
	exec := newChainExecutor(lgr, runner, nil)

	chain := CommandChain{Name: "c1"}
	chain.Add(Command{Name: "p1", Cmd: "echo", Pipe: true})
	chain.Add(Command{Name: "p2", Cmd: "echo", Pipe: true})

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- exec.ExecuteParallel(ctx, []CommandChain{chain})
	}()

	// give some time for goroutines to start
	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-done:
		t.Fatalf("execution finished too early: %v", err)
	default:
	}

	close(pipeBlock1)
	// still should not finish until second is closed
	select {
	case <-done:
		t.Fatalf("execution finished before all piped completed")
	case <-time.After(100 * time.Millisecond):
		// ok
	}

	close(pipeBlock2)

	if err := <-done; err != nil {
		t.Fatalf("ExecuteParallel returned error: %v", err)
	}
}
