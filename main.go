package main

import (
	"context"
	"github.com/efureev/reggol"
	"log"
	"os/signal"
	"syscall"

	parallel "github.com/efureev/parallel/src"
)

func setupContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
}

func initializeApp(configPath string, logger *reggol.Logger) (*parallel.Flow, error) {
	loader := parallel.NewFileLoader(parallel.YamlFileMarshaller{}, logger)
	flow, err := loader.Load(configPath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to load configuration")
		return nil, err
	}
	return &flow, nil
}

func runApplication(ctx context.Context, flags *parallel.Config, logger *reggol.Logger) error {
	flow, err := initializeApp(flags.ConfigFilePath, logger)
	if err != nil {
		return err
	}

	manager := parallel.NewManager(logger)
	if err := manager.ExecuteParallel(ctx, flow.Chains); err != nil {
		logger.Error().Err(err).Msg("Failed to run parallel execution")
		return err
	}

	<-ctx.Done()
	logger.Debug().Msg("App Finished")
	return nil
}

func main() {
	flags, err := parallel.ParseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	ctx, stop := setupContext()
	defer stop()

	logger := parallel.Logger()

	if err := runApplication(ctx, flags, logger); err != nil {
		logger.Error().Err(err).Msg("Application failed")
		return
	}
}
