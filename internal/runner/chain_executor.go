package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

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

	// results заполняется в конце ExecuteParallel и читается уже после её
	// возврата, поэтому синхронизации не требует: запись всех горутин
	// упорядочена относительно чтения вызовом group.Wait.
	results []ChainResult
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
	// проблеме за прогон. От errgroup берём другое: отмену groupCtx при первом
	// отказе и корректное ожидание всех горутин.
	//
	// Слот на цепочку, а не общий срез с мьютексом: порядок ошибок обязан
	// повторять порядок цепочек в конфигурации. Иначе он зависел бы от того,
	// кто раньше упал, а от порядка зависит код возврата утилиты.
	errs := make([]error, len(chains))
	durations := make([]time.Duration, len(chains))
	interrupted := make([]bool, len(chains))

	for i, chain := range chains {
		group.Go(func() error {
			startedAt := time.Now()
			wasStopped, err := c.executeChain(groupCtx, chain)

			durations[i] = time.Since(startedAt)
			errs[i] = err
			interrupted[i] = wasStopped

			return err
		})
	}

	_ = group.Wait()

	c.results = collectResults(chains, errs, durations, interrupted)

	return joinRealErrors(errs)
}

// collectResults сводит исход каждой цепочки в один срез в порядке объявления.
//
// Отмена контекста ошибкой не считается и здесь: цепочка, остановленная из-за
// отказа соседней, в сводке должна выглядеть остановленной, а не упавшей —
// иначе один отказ выглядит как пять.
func collectResults(
	chains []*flow.CommandChain, errs []error, durations []time.Duration, interrupted []bool,
) []ChainResult {
	results := make([]ChainResult, len(chains))

	for i, chain := range chains {
		err := errs[i]
		if errors.Is(err, context.Canceled) {
			err = nil
		}

		results[i] = ChainResult{
			Name:     chain.Name,
			Err:      err,
			Duration: durations[i],
			Stopped:  interrupted[i],
		}
	}

	return results
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
func (c *chainExecutor) executeChain(ctx context.Context, chain *flow.CommandChain) (stopped bool, err error) {
	var (
		piped    errgroup.Group
		firstErr error
	)

	commands := chain.Commands()

	// Слот на команду по той же причине, что и в ExecuteParallel: errgroup.Wait
	// вернула бы только первую ошибку, и отказ второй pipe-команды потерялся бы.
	pipedErrs := make([]error, len(commands))

	for i, cmd := range commands {
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
				err := c.runner.ExecuteWithPipe(ctx, chain, cmd)
				pipedErrs[i] = err

				return err
			})

			continue
		}

		if err := c.runner.Execute(ctx, chain, cmd); err != nil {
			firstErr = err

			break
		}
	}

	_ = piped.Wait()

	// Ошибка последовательной команды идёт первой: именно она оборвала цепочку.
	// Ошибки pipe-команд добавляются следом в порядке объявления.
	joined := joinChainErrors(firstErr, joinRealErrors(pipedErrs))

	// Отмена — не отказ, но и не успех: цепочку остановил отказ соседней либо
	// сигнал. Показать её в сводке как «ok» значило бы выдать убитое за
	// доработавшее, а именно на сводку и смотрят, когда что-то пошло не так.
	return ctx.Err() != nil, joined
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
