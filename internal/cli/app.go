package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/efureev/parallel/internal/buildinfo"
	"github.com/efureev/parallel/internal/config"
	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/runner"
	"github.com/efureev/parallel/internal/ui"
)

const shutdownGraceTimeout = 15 * time.Second

// Коды возврата утилиты.
const (
	exitSuccess = 0
	exitFailure = 1
)

// resolveConfigPath возвращает путь к конфигурации: заданный флагом -f как есть
// либо найденный подъёмом по дереву каталогов.
//
// Явный путь не ищется: опечатка в -f должна давать ошибку, а не приводить
// к тихому запуску чужой конфигурации из родительского каталога.
func resolveConfigPath(configPath string, logger ui.Logger) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determining current directory: %w", err)
	}

	found, err := config.Discover(cwd)
	if err != nil {
		return "", err
	}

	// Найденный не под ногами файл стоит показать: от его расположения зависит
	// разрешение относительных путей в dir.
	if filepath.Dir(found) != cwd {
		logger.Debug("Using configuration from a parent directory", ui.F("path", found))
	}

	return found, nil
}

// runPlan — то, что утилита собирается выполнить: сами цепочки и политика
// запуска.
//
// Политика едет отдельно от Flow намеренно: «останавливать ли соседей при
// отказе» — свойство запуска, а не конфигурации команд, и домену о нём знать
// незачем.
type runPlan struct {
	flow      flow.Flow
	keepGoing bool
}

// loadFlow собирает план: либо из команд, переданных после `--`, либо из файла
// конфигурации.
func loadFlow(flags *Config, logger ui.Logger) (runPlan, error) {
	if len(flags.AdHoc) > 0 {
		adHoc, err := config.AdHoc(flags.AdHoc)

		// Файла нет, значит и ключа failFast быть не может — решает только флаг.
		return runPlan{flow: adHoc, keepGoing: resolveKeepGoing(flags, nil)}, err
	}

	resolved, err := resolveConfigPath(flags.ConfigFilePath, logger)
	if err != nil {
		logger.Error(err, "Failed to locate configuration file")

		return runPlan{}, err
	}

	configData, err := config.NewFileLoader(config.YamlFileMarshaller{}).Load(resolved)
	if err != nil {
		logger.Error(err, "Failed to load configuration file")

		return runPlan{}, err
	}

	for _, hint := range configData.TopLevelHints {
		logger.Warn(hint)
	}

	built, err := config.NewFlowBuilder().Build(configData)

	return runPlan{flow: built, keepGoing: resolveKeepGoing(flags, configData.FailFast)}, err
}

// resolveKeepGoing сводит флаг командной строки и ключ конфигурации в одно
// решение.
//
// Явный флаг сильнее файла: он относится к конкретному запуску, а файл — к
// проекту. Отсюда же и `-keep-going=false` как способ вернуть fail-fast, когда
// в конфигурации стоит `failFast: false`.
func resolveKeepGoing(flags *Config, failFast *bool) bool {
	if flags.KeepGoingSet {
		return flags.KeepGoing
	}

	if failFast != nil {
		return !*failFast
	}

	return false
}

func initializeApp(flags *Config, logger ui.Logger) (*runPlan, error) {
	plan, err := loadFlow(flags, logger)
	if err != nil {
		logger.Error(err, "Invalid configuration")

		return nil, err
	}

	result := plan.flow

	// Отбор идёт до валидации: она обязана относиться к тому, что реально
	// запустится, иначе исключённая цепочка мешала бы запуску остальных.
	result, err = flow.Select(result, flags.Chains, flags.Except)
	if err != nil {
		logger.Error(err, "Invalid chain selection")

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

	plan.flow = result

	return &plan, nil
}

// preview обслуживает режимы, которые ничего не запускают, и сообщает, надо ли
// на этом остановиться.
func preview(flags *Config, result *flow.Flow, logger ui.Logger) (done bool) {
	reader := ui.NewFlowReader(logger)

	if flags.List {
		reader.List(result)

		return true
	}

	reader.Out(result)

	// Предпросмотр уже напечатан — в режиме --dry-run это и есть весь результат.
	if flags.DryRun {
		logger.Info("Dry run: nothing was started")

		return true
	}

	return false
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
	plan, err := initializeApp(flags, logger)
	if err != nil {
		return err
	}

	logger.Debug("Config was loaded...")

	if preview(flags, &plan.flow, logger) {
		return nil
	}

	var managerOpts []runner.Option
	if plan.keepGoing {
		managerOpts = append(managerOpts, runner.WithKeepGoing())
	}

	manager := runner.NewManager(logger, formatter, managerOpts...)

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
		done <- manager.ExecuteParallel(ctx, plan.flow.Chains)
	}()

	waitErr := waitForCompletion(ctx, done, logger)

	// Сводка печатается и при отказе, и при остановке по сигналу: именно тогда
	// она и нужна — понять, какая из цепочек не доехала.
	ui.PrintSummary(logger, summaryRows(manager.Results(), ctx.Err() != nil))

	if waitErr != nil {
		return waitErr
	}

	logger.Debug("App Finished")

	return nil
}

// summaryRows переводит исход цепочек в строки сводки.
//
// Решение о статусе принимается здесь, а не в ui: слой представления не должен
// знать ни про ошибки исполнения, ни про то, что означает отмена контекста.
func summaryRows(results []runner.ChainResult, interrupted bool) []ui.SummaryRow {
	rows := make([]ui.SummaryRow, 0, len(results))

	for _, res := range results {
		row := ui.SummaryRow{Name: res.Name, Status: ui.StatusOK, Duration: res.Duration}

		switch {
		case res.Failed():
			row.Status, row.Reason = ui.StatusFailed, res.Err.Error()
		case res.Stopped || interrupted:
			// Цепочка не упала, но и до конца не дошла: её остановил сигнал.
			row.Status = ui.StatusStopped
		}

		rows = append(rows, row)
	}

	return rows
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
		// Справка уже напечатана разборщиком: это не сбой, а выполненная просьба.
		if errors.Is(err, ErrHelpRequested) {
			return 0
		}

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

	outOpts := []ui.Option{ui.WithLevel(flags.LogLevel)}
	if flags.NoColor {
		outOpts = append(outOpts, ui.WithoutColor())
	}

	out := ui.NewStdoutOutput(outOpts...)
	// Досбрасываем буфер вывода перед выходом: иначе последние строки
	// останутся в буфере и до пользователя не дойдут.
	defer func() { _ = out.Close() }()

	logger, formatter := out.Logger(), out.Formatter()

	// Ошибка уже залогирована там, где возникла, вместе с контекстом. Повторять
	// её здесь значит печатать одно и то же дважды — с длинными сообщениями
	// разбора конфигурации это особенно заметно.
	//
	// Код возврата: наружу уходит собственный код команды, чей отказ остановил
	// запуск, — скрипту важно именно это значение. Ошибки конфигурации и запуска
	// кода команды не имеют и дают exitFailure.
	if err := runApplication(context.Background(), sigCh, flags, logger, formatter); err != nil {
		return runner.ExitCode(err, exitFailure)
	}

	return exitSuccess
}
