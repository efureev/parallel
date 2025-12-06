package parallel

import (
	"context"
	"sync/atomic"
	"testing"
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
