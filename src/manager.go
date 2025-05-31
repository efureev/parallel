package parallel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

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

type CommandExecutor interface {
	Execute(ctx context.Context, command Command) error
	ExecuteWithPipe(ctx context.Context, command Command) error
	ExecuteParallel(ctx context.Context, chains []CommandChain) error
}

type outputHandler func(chainNameStyleText, cmdName, content string, counter int)

type manager struct {
	lgr *reggol.Logger
}

var (
	instance *manager
	mu       sync.Mutex
)

func NewManager(logger *reggol.Logger) CommandExecutor {
	mu.Lock()
	defer mu.Unlock()

	if instance == nil {
		instance = &manager{
			lgr: logger,
		}
	}
	return instance
}

type commandOutput struct {
	chainName string
	cmdName   string
	content   string
	counter   int
}

func (m *manager) formatChainInfo(cmd Command) *commandOutput {
	chain := cmd.GetChain()
	return &commandOutput{
		chainName: strings.ToUpper(chain.Name),
		cmdName:   nameReplace(cmd),
	}
}

func (m *manager) Execute(ctx context.Context, command Command) error {
	cmd := exec.Command(command.Cmd)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()

	stdout, err := cmd.CombinedOutput()
	if err != nil {
		m.lgr.Err(err).Push()
		return fmt.Errorf("%w: %v", ErrCommandExecution, err)
	}

	output := m.formatChainInfo(command)
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

func (m *manager) ExecuteWithPipe(ctx context.Context, command Command) error {
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()

	stdout, stderr, err := setupPipes(cmd)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPipeCreation, err)
	}
	defer stdout.Close()
	defer stderr.Close()

	if err := cmd.Start(); err != nil {
		m.lgr.Error().AnErr("Failed starting command", err).Push()
		return fmt.Errorf("%w: %v", ErrCommandExecution, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	stdoutHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(dividerSymbol)
		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, counter, div)
		m.lgr.Log().Blocks(chainNameStyleText, cmdNameStyled, content).Push()
	}

	stderrHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		m.lgr.Err(errors.New(content)).Blocks(chainNameStyleText, cmdName).Push()
	}

	go func() {
		defer wg.Done()
		m.handleOutput(bufio.NewReader(stdout), command, stdoutHandler)
	}()

	go func() {
		defer wg.Done()
		m.handleOutput(bufio.NewReader(stderr), command, stderrHandler)
	}()

	wg.Wait()

	return handleCommandCompletion(cmd, m.lgr)
}

func (m *manager) handleOutput(reader *bufio.Reader, cmd Command, handler outputHandler) error {
	chain := cmd.GetChain()
	chainName := strings.ToUpper(chain.Name)
	chainNameStyle := chain.Color
	div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(dividerSymbol)
	chainNameStyleText := chainNameStyle.Wrap(chainName) + ` ` + div
	cmdName := nameReplace(cmd)

	counter := 0
	for {
		str, err := reader.ReadString('\n')
		if len(str) == 0 && err != nil {
			if err == io.EOF {
				break
			}
			m.lgr.Err(err).Push()
			return err
		}

		str = strings.TrimSuffix(str, newlineChar)
		handler(chainNameStyleText, cmdName, str, counter)
		counter++

		if err != nil {
			if err == io.EOF {
				break
			}
			m.lgr.Err(err).Push()
			return err
		}
	}
	return nil
}

func handleCommandCompletion(cmd *exec.Cmd, logger *reggol.Logger) error {
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				logger.Error().Int("Exit Status", status.ExitStatus()).Msg("Command failed")
				return fmt.Errorf("%w: exit status %d", ErrCommandExecution, status.ExitStatus())
			}
		}

		return fmt.Errorf("%w: %v", ErrCommandExecution, err)
	}

	return nil
}

func (m *manager) ExecuteParallel(ctx context.Context, chains []CommandChain) error {
	var wg sync.WaitGroup

	wg.Add(len(chains))
	errs := make(chan error, len(chains))

	for _, chain := range chains {
		go func(ch CommandChain) {
			defer wg.Done()

			if err := m.executeChain(ctx, ch); err != nil {
				errs <- err
			}
		}(chain)
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *manager) executeChain(ctx context.Context, chain CommandChain) error {
	for _, cmd := range chain.commands {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if cmd.Pipe {
				if err := m.ExecuteWithPipe(ctx, cmd); err != nil {
					return err
				}
			} else {
				if err := m.Execute(ctx, cmd); err != nil {
					return err
				}
			}
		}
	}
	return nil
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
		stdout.Close() // Clean up the first pipe if second fails
		return nil, nil, fmt.Errorf("failed creating stderr pipe: %w", err)
	}

	return stdout, stderr, nil
}
