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

// executeChain выполняет команды одной цепочки последовательно.
func (c *chainExecutor) executeChain(ctx context.Context, chain CommandChain) error {
	for _, cmd := range chain.commands {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if cmd.Disable {
				c.lgr.Info().Msg(fmt.Sprintf("Command is disabled, skipping: chain=%s command=%s", chain.Name, cmd.getName()))

				continue
			}

			var err error
			if cmd.Pipe {
				err = c.runner.ExecuteWithPipe(ctx, cmd)
			} else {
				err = c.runner.Execute(ctx, cmd)
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}
