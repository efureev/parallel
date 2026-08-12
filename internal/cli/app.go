package cli

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/efureev/parallel/internal/buildinfo"
	"github.com/efureev/parallel/internal/config"
	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/runner"
	"github.com/efureev/parallel/internal/ui"
)

const shutdownGraceTimeout = 15 * time.Second

func initializeApp(configPath string, logger ui.Logger) (*flow.Flow, error) {
	loader := config.NewFileLoader(config.YamlFileMarshaller{})

	configData, err := loader.Load(configPath)
	if err != nil {
		logger.Error(err, "Failed to load configuration file")

		return nil, err
	}

	result, err := config.NewFlowBuilder().Build(configData)
	if err != nil {
		logger.Error(err, "Invalid configuration")

		return nil, err
	}

	if err := result.Validate(); err != nil {
		logger.Error(err, "Invalid flow configuration")

		return nil, err
	}

	// Несуществующий рабочий каталог — почти всегда опечатка, но не обязательно
	// ошибка: каталог может создаваться предыдущей командой. Поэтому
	// предупреждаем, а не отказываемся запускаться.
	for _, md := range flow.MissingDirs(&result) {
		logger.Warn("Working directory does not exist",
			ui.F("chain", md.Chain), ui.F("command", md.Command), ui.F("dir", md.Dir))
	}

	logger.Debug("Config Parsed")

	return &result, nil
}

// runApplication поднимает конфигурацию, запускает выполнение и обслуживает
// сигналы завершения.
func runApplication(
	ctx context.Context,
	sigCh <-chan os.Signal,
	flags *Config,
	logger ui.Logger,
	formatter *ui.OutputFormatter,
) error {
	result, err := initializeApp(flags.ConfigFilePath, logger)
	if err != nil {
		return err
	}

	logger.Debug("Config was loaded...")

	ui.NewFlowReader(logger).Out(result)

	manager := runner.NewManager(logger, formatter)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Лестница реакций на сигналы: вежливо → жёстко → немедленно.
	// Наблюдатель завершается вместе с ctx, а не живёт до конца процесса.
	go watchSignals(ctx, sigCh, signalHandler{
		onFirst: func(sig os.Signal) {
			logger.Info("Shutdown signal received", ui.F("signal", sig.String()))
			manager.SetShutdownSignal(sig)
			cancel()
		},
		onSecond: func() {
			logger.Warn("Second signal received, killing all commands now")
			manager.KillAll()
		},
		exit: func(code int) {
			logger.Warn("Third signal received, exiting immediately")
			os.Exit(code)
		},
	})

	done := make(chan error, 1)

	go func() {
		done <- manager.ExecuteParallel(ctx, result.Chains)
	}()

	if err := waitForCompletion(ctx, done, logger); err != nil {
		return err
	}

	logger.Debug("App Finished")

	return nil
}

// waitForCompletion ждёт окончания выполнения либо отмены по сигналу.
func waitForCompletion(ctx context.Context, done <-chan error, logger ui.Logger) error {
	select {
	case err := <-done:
		if err != nil {
			logger.Error(err, "Failed to run parallel execution")

			return err
		}

		logger.Info("All commands completed successfully")

		return nil

	case <-ctx.Done():
		logger.Info("Shutdown signal received, waiting for commands to stop...")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGraceTimeout)
		defer cancel()

		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error(err, "Error during shutdown")

				return err
			}

			logger.Info("All commands stopped gracefully")
		case <-shutdownCtx.Done():
			logger.Warn("Shutdown timeout reached, some commands may have been force-killed")
		}

		return nil
	}
}

// Run содержит основную логику и возвращает код выхода процесса,
// чтобы defer-ы отработали до os.Exit.
func Run() int {
	flags, err := ParseFlags()
	if err != nil {
		log.Printf("Failed to parse flags: %v", err)

		return 1
	}

	// Handle version request early and exit.
	if flags.VersionRequested {
		log.Print(buildinfo.Long())

		return 0
	}

	sigCh, stopNotify := notifyShutdown()
	defer stopNotify()

	out := ui.NewStdoutOutput()
	// Досбрасываем буфер вывода перед выходом: иначе последние строки
	// останутся в буфере и до пользователя не дойдут.
	defer func() { _ = out.Close() }()

	logger, formatter := out.Logger(), out.Formatter()

	// Ошибка уже залогирована там, где возникла, вместе с контекстом. Повторять
	// её здесь значит печатать одно и то же дважды — с длинными сообщениями
	// разбора конфигурации это особенно заметно.
	if err := runApplication(context.Background(), sigCh, flags, logger, formatter); err != nil {
		return 1
	}

	return 0
}
