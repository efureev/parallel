package runner

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// TestExecuteParallel_NoGoroutineLeak.
//
// Раньше ExecuteParallel запускал горутину-наблюдателя, которая ждала отмены
// РОДИТЕЛЬСКОГО контекста. Если тот не отменялся никогда — а в тестах и при
// использовании пакета как библиотеки это обычное дело, — горутина оставалась
// жить до конца процесса вместе с замыканием на stopAll.
//
// Контекст здесь намеренно НЕ отменяется: именно этот сценарий и протекал.
func TestExecuteParallel_NoGoroutineLeak(t *testing.T) {
	out := ui.NewDiscardOutput()
	lgr, formatter := out.Logger(), out.Formatter()
	mgr := NewManager(lgr, formatter, WithTimeouts(testTimeouts))

	makeChains := func() []*flow.CommandChain {
		chains := make([]*flow.CommandChain, 0, 4)

		for i := range 4 {
			chain := &flow.CommandChain{Name: "c" + string(rune('0'+i))}
			chain.Add(flow.Command{Name: "noop", Cmd: "echo", Args: []string{"hi"}})
			chains = append(chains, chain)
		}

		return chains
	}

	// Прогреваем: первый запуск создаёт ленивые внутренности рантайма,
	// и считать горутины до него бессмысленно.
	if err := mgr.ExecuteParallel(context.Background(), makeChains()); err != nil {
		t.Fatalf("warm-up run: %v", err)
	}

	before := stableGoroutineCount()

	for range 10 {
		if err := mgr.ExecuteParallel(context.Background(), makeChains()); err != nil {
			t.Fatalf("ExecuteParallel: %v", err)
		}
	}

	after := stableGoroutineCount()

	// Допуск на горутины рантайма, которые могут появиться не по нашей вине.
	const tolerance = 2

	if after > before+tolerance {
		t.Errorf("горутины утекают: было %d, стало %d после 10 прогонов", before, after)
	}
}

// stableGoroutineCount ждёт, пока число горутин перестанет меняться.
// Завершение горутины не мгновенно, и без этого счёт был бы гонкой.
func stableGoroutineCount() int {
	prev := -1

	for range 50 {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)

		if n := runtime.NumGoroutine(); n == prev {
			return n
		} else {
			prev = n
		}
	}

	return runtime.NumGoroutine()
}
