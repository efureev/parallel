package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/efureev/reggol"

	parallel "github.com/efureev/parallel/src"
)

const shutdownGraceTimeout = 15 * time.Second

// setupSignalContext создает контекст, отменяемый по сигналу, и канал для самих сигналов.
func setupSignalContext() (context.Context, context.CancelFunc, <-chan os.Signal) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	return ctx, cancel, sigCh
}

func initializeApp(configPath string, logger *reggol.Logger) (*parallel.Flow, error) {
	loader := parallel.NewFileLoader(parallel.YamlFileMarshaller{})

	configData, err := loader.Load(configPath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to load configuration file")

		return nil, err
	}

	builder := parallel.NewFlowBuilder(logger)
	flow := builder.Build(configData)

	if err := flow.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid flow configuration")

		return nil, err
	}

	logger.Debug().Msg("Config Parsed")

	return &flow, nil
}

//nolint:funlen // Orchestrates signals, context, and execution; acceptable length for clarity.
func runApplication(ctx context.Context, sigCh <-chan os.Signal, flags *parallel.Config, logger *reggol.Logger) error {
	flow, err := initializeApp(flags.ConfigFilePath, logger)
	if err != nil {
		return err
	}

	logger.Debug().Msg("Config was loaded...")

	flowReader := parallel.NewFlowReader(logger)
	flowReader.Out(flow)

	manager := parallel.NewManager(logger)

	// Запуск мониторинга сигналов: первый сигнал сохраняем в менеджер и отменяем контекст.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if sig, ok := (<-sigCh).(syscall.Signal); ok {
			logger.Info().Str("signal", sig.String()).Msg("Shutdown signal received")
			manager.SetShutdownSignal(sig)
			cancel()
		}
	}()

	// Запускаем выполнение в отдельной горутине
	done := make(chan error, 1)

	go func() {
		done <- manager.ExecuteParallel(ctx, flow.Chains)
	}()

	// Ждем завершения или отмены
	select {
	case err := <-done:
		if err != nil {
			logger.Error().Err(err).Msg("Failed to run parallel execution")

			return err
		}

		logger.Info().Msg("All commands completed successfully")
	case <-ctx.Done():
		logger.Info().Msg("Shutdown signal received, waiting for commands to stop...")

		// Даем время на graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGraceTimeout)
		defer cancel()

		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error().Err(err).Msg("Error during shutdown")

				return err
			}

			logger.Info().Msg("All commands stopped gracefully")
		case <-shutdownCtx.Done():
			logger.Warn().Msg("Shutdown timeout reached, some commands may have been force-killed")
		}
	}

	logger.Debug().Msg("App Finished")

	return nil
}

func main() {
	flags, err := parallel.ParseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	ctx, cancel, sigCh := setupSignalContext()
	defer cancel()

	logger := parallel.Logger()

	if err := runApplication(ctx, sigCh, flags, logger); err != nil {
		logger.Error().Err(err).Msg("Application failed")

		return
	}
}
