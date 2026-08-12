package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// CommandRunner описывает низкоуровневое выполнение одной команды.
type CommandRunner interface {
	Execute(ctx context.Context, chain *flow.CommandChain, command flow.Command) error
	ExecuteWithPipe(ctx context.Context, chain *flow.CommandChain, command flow.Command) error
}

// stopAllFunc используется для остановки всех запущенных процессов при завершении.
type stopAllFunc func()

// chainExecutor отвечает за выполнение цепочек команд поверх низкоуровневого раннера.
type chainExecutor struct {
	lgr     ui.Logger
	runner  CommandRunner
	stopAll stopAllFunc
}

func newChainExecutor(lgr ui.Logger, runner CommandRunner, stopAll stopAllFunc) *chainExecutor {
	return &chainExecutor{
		lgr:     lgr,
		runner:  runner,
		stopAll: stopAll,
	}
}

// ExecuteParallel выполняет цепочки параллельно, а команды внутри одной цепочки —
// с учётом их pipe-флага.
//
// Оркестрация построена на errgroup: раньше здесь были собственные WaitGroup,
// канал ошибок и горутина-монитор, причём монитор ждал отмены РОДИТЕЛЬСКОГО
// контекста и жил до конца процесса, если тот не отменялся никогда.
// errgroup.WithContext закрывает и оркестрацию, и время жизни наблюдателя.
func (c *chainExecutor) ExecuteParallel(ctx context.Context, chains []*flow.CommandChain) error {
	group, groupCtx := errgroup.WithContext(ctx)

	// Наблюдатель за отменой снаружи: живёт ровно столько же, сколько группа,
	// потому что ждёт groupCtx, а тот закрывается при выходе из Wait.
	stopped := make(chan struct{})
	defer close(stopped)

	go c.watchCancellation(ctx, stopped)

	// Ошибки собираются отдельно от errgroup намеренно: errgroup.Wait отдаёт
	// только первую, а нам нужны все — иначе пользователь чинит по одной
	// проблеме за прогон. От errgroup берём другое: отмену
	// groupCtx при первом отказе и корректное ожидание всех горутин.
	var (
		mu   sync.Mutex
		errs []error
	)

	for _, chain := range chains {
		group.Go(func() error {
			err := c.executeChain(groupCtx, chain)
			if err != nil {
				mu.Lock()

				errs = append(errs, err)

				mu.Unlock()
			}

			return err
		})
	}

	_ = group.Wait()

	return joinRealErrors(errs)
}

// joinRealErrors объединяет ошибки цепочек, отбрасывая отмену контекста:
// цепочка, остановленная из-за отказа соседней, сбоем не является.
func joinRealErrors(errs []error) error {
	real := make([]error, 0, len(errs))

	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			real = append(real, err)
		}
	}

	return errors.Join(real...)
}

// watchCancellation останавливает все процессы, когда снаружи отменяют контекст.
//
// Завершается либо по отмене, либо по закрытию stopped — то есть не переживает
// вызов ExecuteParallel даже если родительский контекст не отменяется никогда.
func (c *chainExecutor) watchCancellation(ctx context.Context, stopped <-chan struct{}) {
	select {
	case <-ctx.Done():
	case <-stopped:
		return
	}

	c.lgr.Info("Shutdown signal received, stopping all commands...")

	if c.stopAll != nil {
		c.stopAll()
	}
}

// logSkipped сообщает о команде, отключённой флагом disable в конфигурации.
func (c *chainExecutor) logSkipped(chain *flow.CommandChain, cmd flow.Command) {
	c.lgr.Info(
		fmt.Sprintf("Command is disabled, skipping: chain=%s command=%s", chain.Name, cmd.DisplayName()),
	)
}

// executeChain выполняет команды одной цепочки с учётом pipe-флага:
//   - pipe=false — выполняются последовательно, в порядке из конфигурации;
//   - pipe=true  — запускаются сразу и работают параллельно, но цепочка не
//     считается завершённой, пока не закончится каждая из них.
//
// Отказ последовательной команды прекращает запуск следующих; уже запущенные
// pipe-команды всё равно дожидаются.
func (c *chainExecutor) executeChain(ctx context.Context, chain *flow.CommandChain) error {
	var (
		piped    errgroup.Group
		firstErr error
	)

	for _, cmd := range chain.Commands() {
		if ctx.Err() != nil {
			firstErr = ctx.Err()

			break
		}

		if cmd.Disable {
			c.logSkipped(chain, cmd)

			continue
		}

		if cmd.Pipe {
			piped.Go(func() error {
				return c.runner.ExecuteWithPipe(ctx, chain, cmd)
			})

			continue
		}

		if err := c.runner.Execute(ctx, chain, cmd); err != nil {
			firstErr = err

			break
		}
	}

	// Ошибка последовательной команды идёт первой: именно она оборвала цепочку.
	// Ошибки pipe-команд добавляются следом, ни одна не теряется.
	return joinChainErrors(firstErr, piped.Wait())
}

// joinChainErrors объединяет ошибку, оборвавшую цепочку, с ошибками pipe-команд,
// отбрасывая отмену контекста: она означает штатную остановку, а не сбой.
func joinChainErrors(firstErr, pipedErr error) error {
	if errors.Is(firstErr, context.Canceled) {
		firstErr = nil
	}

	if errors.Is(pipedErr, context.Canceled) {
		pipedErr = nil
	}

	return errors.Join(firstErr, pipedErr)
}
