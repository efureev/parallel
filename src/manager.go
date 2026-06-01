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
	ExecuteParallel(ctx context.Context, chains []*CommandChain) error
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

// supervise регистрирует процесс в registry, ждёт его завершения либо отмены контекста,
// отправляет сигнал завершения группе и принудительно убивает её по таймауту.
// Владельцем cmd.Wait() является именно эта функция: результат отдаётся через канал,
// а waitDone закрывается ровно один раз; registry лишь ждёт waitDone и сам Wait не вызывает.
//
// onCancel (если задан) вызывается сразу при обнаружении отмены контекста — например,
// чтобы остановить чтение вывода. onDone (если задан) вызывается после полного выхода
// процесса в обеих ветках — например, чтобы дождаться завершения output-горутин.
//
// Возвращает ошибку cmd.Wait() при штатном завершении либо ctx.Err() при отмене.
func (m *manager) supervise(ctx context.Context, cmd *exec.Cmd, command Command, onCancel, onDone func()) error {
	cmdKey := uniqueCmdKey(command, cmd.Process.Pid)

	waitErr := make(chan error, 1)
	waitDone := make(chan struct{})

	m.procs.add(cmdKey, cmd, waitDone)
	defer m.procs.remove(cmdKey)

	go func() {
		waitErr <- cmd.Wait()

		close(waitDone)
	}()

	select {
	case <-ctx.Done():
		m.lgr.Info().Str("cmd", command.Cmd).Msg("Context canceled, stopping command")

		if onCancel != nil {
			onCancel()
		}

		if err := sendSignalToGroup(cmd, m.getShutdownSignal()); err != nil {
			m.lgr.Warn().Err(err).Str("cmd", command.Cmd).Msg("Failed to send shutdown signal to process group")
		}

		select {
		case <-waitDone:
		case <-time.After(forceKillTimeout):
			m.lgr.Warn().Str("cmd", command.Cmd).Msg("Force killing command group")

			if err := killProcessGroup(cmd); err != nil {
				m.lgr.Warn().Err(err).Str("cmd", command.Cmd).Msg("Failed to kill process group")
			}

			<-waitDone
		}

		if onDone != nil {
			onDone()
		}

		return ctx.Err()

	case <-waitDone:
		if onDone != nil {
			onDone()
		}

		return <-waitErr
	}
}

func (m *manager) Execute(ctx context.Context, command Command) error {
	//nolint:gosec // command/args come from trusted config for CLI tool
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()
	configureProcessGroup(cmd)

	var stdoutBuf, stderrBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		m.lgr.Err(err).Msg("Failed to start command")

		return fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	m.lgr.Info().Msg(fmt.Sprintf("Command started: %s", fullDisplayName(command)))

	if err := m.supervise(ctx, cmd, command, nil, nil); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		m.lgr.Err(err).Push()

		return fmt.Errorf("%w: %w", ErrCommandExecution, err)
	}

	m.printBlock(command, stdoutBuf.Bytes(), stderrBuf.Bytes())

	return nil
}

// printBlock печатает результат не-pipe команды, сохраняя разделение потоков:
// stdout выводится обычным блоком, непустой stderr — отдельным блоком ошибки.
func (m *manager) printBlock(command Command, stdout, stderr []byte) {
	output := m.output.formatChainInfo(command)

	var chainHeader string
	if chain := command.GetChain(); chain != nil {
		chainHeader = chain.Color.Wrap(output.chainName + dividerSymbol)
	} else {
		chainHeader = output.chainName + dividerSymbol
	}

	if len(stdout) > 0 {
		m.lgr.Log().Blocks(chainHeader, output.cmdName, indentBlock(stdout)).Push()
	}

	if len(stderr) > 0 {
		m.lgr.Err(errors.New(indentBlock(stderr))).Blocks(chainHeader, output.cmdName).Push()
	}
}

// indentBlock форматирует многострочный вывод с отступом для читаемости.
func indentBlock(b []byte) string {
	lines := strings.Split(string(b), newlineChar)
	content := newlineChar

	for _, msg := range lines {
		content += outputIndentation + msg + newlineChar
	}

	return content
}

func (m *manager) ExecuteWithPipe(ctx context.Context, command Command) error {
	//nolint:gosec // command/args come from trusted config for CLI tool
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()

	// Настраиваем process group для правильной передачи сигналов
	configureProcessGroup(cmd)

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

	m.lgr.Info().Msg(fmt.Sprintf("Command started: %s", fullDisplayName(command)))

	// Контекст для отмены чтения вывода.
	// Чтение запускается до supervise, чтобы корректно дочитать пайпы.
	outputCtx, outputCancel := context.WithCancel(ctx)
	defer outputCancel()

	wg := m.streamPipes(outputCtx, command, stdout, stderr)

	// onCancel останавливает чтение вывода сразу при отмене; onDone дожидается
	// завершения output-горутин после полного выхода процесса.
	onDone := func() {
		outputCancel()
		wg.Wait()
	}

	if err := m.supervise(ctx, cmd, command, outputCancel, onDone); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		return handleCommandCompletionErr(err, m.lgr)
	}

	return nil
}

// streamPipes запускает две горутины чтения stdout/stderr и возвращает WaitGroup,
// по которой можно дождаться завершения обработки вывода.
func (m *manager) streamPipes(
	ctx context.Context,
	command Command,
	stdout, stderr io.ReadCloser,
) *sync.WaitGroup {
	var wg sync.WaitGroup

	stdoutHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(dividerSymbol)
		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, counter, div)
		m.lgr.Log().Blocks(chainNameStyleText, cmdNameStyled, content).Push()
	}

	stderrHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		m.lgr.Err(errors.New(content)).Blocks(chainNameStyleText, cmdName).Push()
	}

	wg.Go(func() {
		if err := m.output.handleOutput(ctx, bufio.NewReader(stdout), command, stdoutHandler); err != nil {
			m.lgr.Err(err).Push()
		}
	})

	wg.Go(func() {
		if err := m.output.handleOutput(ctx, bufio.NewReader(stderr), command, stderrHandler); err != nil {
			m.lgr.Err(err).Push()
		}
	})

	return &wg
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

func (m *manager) ExecuteParallel(ctx context.Context, chains []*CommandChain) error {
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

// fullDisplayName returns a human-friendly display string with chain name
// included when available, e.g. "CHAIN > echo hello".
func fullDisplayName(cmd Command) string {
	chain := cmd.GetChainName()
	if chain == "" {
		return nameReplace(cmd)
	}

	return fmt.Sprintf("%s > %s", chain, nameReplace(cmd))
}

// uniqueCmdKey builds a unique key for the process registry that includes
// the chain and command names along with the PID to avoid collisions.
func uniqueCmdKey(cmd Command, pid int) string {
	base := cmd.getName()

	chain := cmd.GetChainName()
	if chain != "" {
		base = chain + "/" + base
	}

	return fmt.Sprintf("%s_%d", base, pid)
}

func setupPipes(cmd *exec.Cmd) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating stdout pipe: %w", err)
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		// Clean up the first pipe if the second fails, but preserve the original cause.
		if closeErr := stdout.Close(); closeErr != nil {
			return nil, nil, errors.Join(fmt.Errorf("failed creating stderr pipe: %w", err), closeErr)
		}

		return nil, nil, fmt.Errorf("failed creating stderr pipe: %w", err)
	}

	return stdout, stderr, nil
}
