package parallel

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/efureev/reggol"
)

// CommandRunner описывает низкоуровневое выполнение одной команды.
type CommandRunner interface {
	Execute(ctx context.Context, command Command) error
	ExecuteWithPipe(ctx context.Context, command Command) error
}

// stopAllFunc используется для остановки всех запущенных процессов при завершении.
type stopAllFunc func()

// chainExecutor отвечает за выполнение цепочек команд поверх низкоуровневого раннера.
type chainExecutor struct {
	lgr     *reggol.Logger
	runner  CommandRunner
	stopAll stopAllFunc
}

func newChainExecutor(lgr *reggol.Logger, runner CommandRunner, stopAll stopAllFunc) *chainExecutor {
	return &chainExecutor{
		lgr:     lgr,
		runner:  runner,
		stopAll: stopAll,
	}
}

// ExecuteParallel выполняет цепочки параллельно, а команды внутри одной цепочки — последовательно.
func (c *chainExecutor) ExecuteParallel(ctx context.Context, chains []CommandChain) error {
	var wg sync.WaitGroup
	wg.Add(len(chains))
	errCh := make(chan error, len(chains))

	// Создаем контекст для мониторинга отмены
	chainCtx, chainCancel := context.WithCancel(ctx)
	defer chainCancel()

	for _, chain := range chains {
		go func(ch CommandChain) {
			defer wg.Done()

			if err := c.executeChain(chainCtx, ch); err != nil {
				if !errors.Is(err, context.Canceled) {
					errCh <- err
				}
			}
		}(chain)
	}

	// Мониторим отмену контекста
	go func() {
		<-ctx.Done()
		c.lgr.Info().Msg("Shutdown signal received, stopping all commands...")

		if c.stopAll != nil {
			c.stopAll()
		}

		chainCancel()
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

// executeChain выполняет команды одной цепочки с учетом pipe-флага:
//   - pipe=false — выполняются последовательно (синхронно)
//   - pipe=true  — запускаются в горутинах и выполняются параллельно, но в рамках цепочки
//     завершение цепочки ожидает окончания всех запущенных pipe-команд
func (c *chainExecutor) executeChain(ctx context.Context, chain CommandChain) error {
	var (
		wg       sync.WaitGroup
		firstErr error
	)

	errCh := make(chan error, len(chain.commands))
	breakLoop := false

	for _, cmd := range chain.commands {
		if breakLoop {
			break
		}

		select {
		case <-ctx.Done():
			firstErr = ctx.Err()
			breakLoop = true
		default:
			if cmd.Disable {
				c.lgr.Info().Msg(fmt.Sprintf("Command is disabled, skipping: chain=%s command=%s", chain.Name, cmd.getName()))

				continue
			}

			if cmd.Pipe { // Параллельный запуск
				wg.Add(1)

				go func(cm Command) {
					defer wg.Done()

					if err := c.runner.ExecuteWithPipe(ctx, cm); err != nil && !errors.Is(err, context.Canceled) {
						errCh <- err
					}
				}(cmd)
			} else { // Последовательный запуск
				if err := c.runner.Execute(ctx, cmd); err != nil {
					firstErr = err
					breakLoop = true
				}
			}
		}
	}

	// Ожидаем завершения всех запущенных pipe-команд
	wg.Wait()
	close(errCh)

	if firstErr != nil && !errors.Is(firstErr, context.Canceled) {
		return firstErr
	}

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	if firstErr != nil {
		return firstErr
	}

	return nil
}
