package runner

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// errFakeA и errFakeB — различимые ошибки, чтобы проверить, что до вызывающего
// доходят обе, а не первая попавшаяся.
var (
	errFakeA = errors.New("chain A failed")
	errFakeB = errors.New("chain B failed")
)

// failingRunner возвращает заранее заданную ошибку для команды с указанным именем.
//
// barrier, если задан, задерживает возврат до тех пор, пока в него не войдут
// все ожидаемые команды. Без него тест на объединение ошибок был бы гонкой:
// отказ одной цепочки отменяет соседние, и вторая могла просто не успеть упасть.
type failingRunner struct {
	byName  map[string]error
	barrier *sync.WaitGroup
}

func (f *failingRunner) run(cmd flow.Command) error {
	if f.barrier != nil {
		f.barrier.Done()
		f.barrier.Wait()
	}

	return f.byName[cmd.Name]
}

func (f *failingRunner) Execute(_ context.Context, _ *flow.CommandChain, cmd flow.Command) error {
	return f.run(cmd)
}

func (f *failingRunner) ExecuteWithPipe(_ context.Context, _ *flow.CommandChain, cmd flow.Command) error {
	return f.run(cmd)
}

// TestExecuteParallel_JoinsAllErrors требует, чтобы до вызывающего дошли все
// ошибки, а не первая попавшаяся: иначе пользователь чинит по одной проблеме
// за прогон, и какая именно долетит — зависит от планировщика.
//
// Барьер гарантирует, что обе цепочки дошли до отказа, — иначе проверялась бы
// не сборка ошибок, а скорость планировщика.
func TestExecuteParallel_JoinsAllErrors(t *testing.T) {
	barrier := &sync.WaitGroup{}
	barrier.Add(2)

	runner := &failingRunner{
		byName:  map[string]error{"a": errFakeA, "b": errFakeB},
		barrier: barrier,
	}

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	chainA := &flow.CommandChain{Name: "A"}
	chainA.Add(flow.Command{Name: "a", Cmd: "echo"})

	chainB := &flow.CommandChain{Name: "B"}
	chainB.Add(flow.Command{Name: "b", Cmd: "echo"})

	err := exec.ExecuteParallel(t.Context(), []*flow.CommandChain{chainA, chainB})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !errors.Is(err, errFakeA) {
		t.Errorf("ошибка первой цепочки потеряна: %v", err)
	}

	if !errors.Is(err, errFakeB) {
		t.Errorf("ошибка второй цепочки потеряна: %v", err)
	}
}

// TestExecuteChain_JoinsSequentialAndPipedErrors проверяет тот же инвариант
// внутри одной цепочки: отказ последовательной команды не должен прятать
// отказ уже запущенной pipe-команды.
func TestExecuteChain_JoinsSequentialAndPipedErrors(t *testing.T) {
	runner := &failingRunner{byName: map[string]error{
		"piped":      errFakeA,
		"sequential": errFakeB,
	}}

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	chain := &flow.CommandChain{Name: "mixed"}
	chain.Add(flow.Command{Name: "piped", Cmd: "echo", Pipe: true})
	chain.Add(flow.Command{Name: "sequential", Cmd: "echo"})

	err := exec.ExecuteParallel(t.Context(), []*flow.CommandChain{chain})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !errors.Is(err, errFakeA) {
		t.Errorf("ошибка pipe-команды потеряна: %v", err)
	}

	if !errors.Is(err, errFakeB) {
		t.Errorf("ошибка последовательной команды потеряна: %v", err)
	}
}

// TestJoinChainErrors_DropsCancellation фиксирует, что штатная остановка по
// отмене контекста не выдаётся за сбой команды.
func TestJoinChainErrors_DropsCancellation(t *testing.T) {
	if err := joinChainErrors(context.Canceled, nil); err != nil {
		t.Errorf("отмена не должна считаться ошибкой, получено %v", err)
	}

	if err := joinChainErrors(nil, nil); err != nil {
		t.Errorf("без ошибок ожидался nil, получено %v", err)
	}

	err := joinChainErrors(errFakeA, context.Canceled)
	if !errors.Is(err, errFakeA) {
		t.Errorf("настоящая ошибка потеряна: %v", err)
	}
}
