package parallel

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/efureev/reggol"
)

const (
	cmdNameTemplate   = "%CMD_NAME%"
	cmdArgsTemplate   = "%CMD_ARGS%"
	outputIndentation = "          "
	dividerSymbol     = ">"
	newlineChar       = "\n"
)

var (
	ErrCommandExecution = errors.New("command execution failed")
	ErrPipeCreation     = errors.New("pipe creation failed")
)

// CommandExecutor описывает внешний API менеджера для выполнения цепочек команд
// и управления сигналом завершения.
type CommandExecutor interface {
	ExecuteParallel(ctx context.Context, chains []CommandChain) error
	SetShutdownSignal(sig syscall.Signal)
}

type manager struct {
	lgr *reggol.Logger

	procs       *processRegistry
	shutdownMu  sync.RWMutex
	shutdownSig syscall.Signal

	output *outputFormatter
	chains *chainExecutor
}

// NewManager создаёт новый экземпляр менеджера.
func NewManager(logger *reggol.Logger) CommandExecutor {
	m := &manager{
		lgr:         logger,
		procs:       newProcessRegistry(),
		shutdownSig: syscall.SIGTERM,
	}

	m.output = newOutputFormatter(logger)
	m.chains = newChainExecutor(logger, m, m.stopAllCommands)

	return m
}

// SetShutdownSignal позволяет задать сигнал, который будет отправляться дочерним процессам
// при завершении работы приложения (например, SIGINT / SIGTERM / SIGQUIT).
func (m *manager) SetShutdownSignal(sig syscall.Signal) {
	m.shutdownMu.Lock()
	defer m.shutdownMu.Unlock()

	m.shutdownSig = sig
}

func (m *manager) getShutdownSignal() syscall.Signal {
	m.shutdownMu.RLock()
	defer m.shutdownMu.RUnlock()

	if m.shutdownSig == 0 {
		return syscall.SIGTERM
	}

	return m.shutdownSig
}

func (m *manager) stopAllCommands() {
	if m.procs == nil {
		return
	}

	m.procs.stopAll(m.lgr, m.getShutdownSignal())
}

//nolint:funlen
func (m *manager) Execute(ctx context.Context, command Command) error {
	//nolint:gosec // command/args come from trusted config for CLI tool
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stdoutBuf

	if err := cmd.Start(); err != nil {
		m.lgr.Err(err).Msg("Failed to start command")

		return fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	m.lgr.Info().Msg(fmt.Sprintf("Command started: %s", nameReplace(command)))

	// Регистрируем команду для корректного shutdown
	cmdKey := fmt.Sprintf("%s_%d", command.Cmd, cmd.Process.Pid)

	m.procs.add(cmdKey, cmd)
	defer m.procs.remove(cmdKey)

	done := make(chan error, 1)

	go func() { done <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		m.lgr.Info().Str("cmd", command.Cmd).Msg("Context canceled, stopping command")

		if err := sendSignalToGroup(cmd, m.getShutdownSignal()); err != nil {
			m.lgr.Warn().Err(err).Str("cmd", command.Cmd).Msg("Failed to send shutdown signal to process group")
		}

		select {
		case <-time.After(forceKillTimeout):
			m.lgr.Warn().Str("cmd", command.Cmd).Msg("Force killing command group")

			if err := killProcessGroup(cmd); err != nil {
				m.lgr.Warn().Err(err).Str("cmd", command.Cmd).Msg("Failed to kill process group")
			}
		case <-done:
		}

		return ctx.Err()

	case err := <-done:
		if err != nil {
			m.lgr.Err(err).Push()

			return fmt.Errorf("%w: %w", ErrCommandExecution, err)
		}
	}

	stdout := stdoutBuf.Bytes()

	output := m.output.formatChainInfo(command)
	lines := strings.Split(string(stdout), newlineChar)
	content := newlineChar

	for _, msg := range lines {
		content += outputIndentation + msg + newlineChar
	}

	m.lgr.Log().Blocks(
		command.GetChain().Color.Wrap(output.chainName+dividerSymbol),
		output.cmdName,
		content,
	).Push()

	return nil
}

//nolint:funlen // function orchestrates IO, signals, and waits; splitting would hurt readability here
func (m *manager) ExecuteWithPipe(ctx context.Context, command Command) error {
	//nolint:gosec // command/args come from trusted config for CLI tool
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()

	// Настраиваем process group для правильной передачи сигналов
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, stderr, err := setupPipes(cmd)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPipeCreation, err)
	}
	defer stdout.Close()
	defer stderr.Close()

	if err := cmd.Start(); err != nil {
		m.lgr.Error().AnErr("Failed starting command", err).Push()

		return fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	m.lgr.Info().Msg(fmt.Sprintf("Command started: %s", nameReplace(command)))

	// Регистрируем команду для отслеживания
	cmdKey := fmt.Sprintf("%s_%d", command.Cmd, cmd.Process.Pid)

	m.procs.add(cmdKey, cmd)
	defer m.procs.remove(cmdKey)

	var wg sync.WaitGroup

	const numOutputGoroutines = 2
	wg.Add(numOutputGoroutines)

	stdoutHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(dividerSymbol)
		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, counter, div)
		m.lgr.Log().Blocks(chainNameStyleText, cmdNameStyled, content).Push()
	}

	stderrHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		m.lgr.Err(errors.New(content)).Blocks(chainNameStyleText, cmdName).Push()
	}

	// Контекст для отмены чтения вывода
	outputCtx, outputCancel := context.WithCancel(ctx)
	defer outputCancel()

	go func() {
		defer wg.Done()

		if err := m.output.handleOutput(outputCtx, bufio.NewReader(stdout), command, stdoutHandler); err != nil {
			m.lgr.Err(err).Push()
		}
	}()

	go func() {
		defer wg.Done()

		if err := m.output.handleOutput(outputCtx, bufio.NewReader(stderr), command, stderrHandler); err != nil {
			m.lgr.Err(err).Push()
		}
	}()

	// Ждем завершения команды или отмены контекста
	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Контекст отменен, завершаем команду
		m.lgr.Info().Str("cmd", command.Cmd).Msg("Context canceled, stopping command")

		// Отменяем чтение вывода
		outputCancel()

		// Отправляем дочернему процессу сигнал завершения (такой же, как получил менеджер)
		if err := sendSignalToGroup(cmd, m.getShutdownSignal()); err != nil {
			m.lgr.Warn().Err(err).Msg("Failed to send shutdown signal to process group")
		}

		// Ждем немного для graceful shutdown
		select {
		case <-done:
			// Процесс завершился
		case <-time.After(forceKillTimeout):
			// Принудительно завершаем всю группу
			m.lgr.Warn().Str("cmd", command.Cmd).Msg("Force killing command group")

			if err := killProcessGroup(cmd); err != nil {
				m.lgr.Warn().Err(err).Str("cmd", command.Cmd).Msg("Failed to kill process group")
			}
		}

		wg.Wait()

		return ctx.Err()

	case err := <-done:
		// Команда завершилась
		outputCancel()
		wg.Wait()

		if err != nil {
			return handleCommandCompletionErr(err, m.lgr)
		}

		return nil
	}
}

func handleCommandCompletionErr(waitErr error, logger *reggol.Logger) error {
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			logger.Error().Int("Exit Status", status.ExitStatus()).Msg("Command failed")

			return fmt.Errorf("%w: exit status %d", ErrCommandExecution, status.ExitStatus())
		}
	}

	return fmt.Errorf("%w: %w", ErrCommandExecution, waitErr)
}

func (m *manager) ExecuteParallel(ctx context.Context, chains []CommandChain) error {
	return m.chains.ExecuteParallel(ctx, chains)
}

func nameReplace(cmd Command) string {
	if cmd.Format.CmdName == `` {
		return fmt.Sprintf(`%s %s`, cmd.getName(), strings.Join(cmd.Args, ` `))
	}

	tlpList := [2]string{cmdNameTemplate, cmdArgsTemplate}
	valueList := [2]string{cmd.getName(), strings.Join(cmd.Args, ` `)}
	result := cmd.Format.CmdName

	for idx, tpl := range tlpList {
		result = strings.ReplaceAll(result, tpl, valueList[idx])
	}

	return result
}

func setupPipes(cmd *exec.Cmd) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating stdout pipe: %w", err)
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		err := stdout.Close()
		if err != nil {
			return nil, nil, err
		} // Clean up the first pipe if second fails

		return nil, nil, fmt.Errorf("failed creating stderr pipe: %w", err)
	}

	return stdout, stderr, nil
}
