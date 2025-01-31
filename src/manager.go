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

type manager struct {
	lgr *reggol.Logger
}

var instance *manager

func Manager(logger *reggol.Logger) *manager {
	if instance == nil {
		instance = &manager{
			lgr: logger,
		}
	}

	return instance
}

func (m *manager) Run(ctx context.Context, command Command) {
	cmd := exec.Command(command.Cmd)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()

	stdout, err := cmd.CombinedOutput()
	if err != nil {
		m.lgr.Err(err).Push()

		return
	}

	strs := strings.Split(string(stdout), "\n")

	chain := command.GetChain()
	chainName := strings.ToUpper(chain.Name)
	chainNameStyle := chain.Color

	cmdName := fmt.Sprintf(`%s %s`, command.getName(), strings.Join(command.Args, ` `))

	content := "\n"

	for _, msg := range strs {
		content += strings.Repeat(` `, 10) + msg + "\n"
	}

	m.lgr.Log().Blocks(chainNameStyle.Wrap(chainName+` > `), cmdName, content).Push()

	// need to stop waiting..
	// ctx.Done <- ...
}

func (m *manager) RunWithPipe(ctx context.Context, command Command) {
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()

	if err != nil {
		m.lgr.Fatal().Msgf("failed creating command stdoutPipe: %s", err)
	}

	defer stdout.Close()

	stdoutReader := bufio.NewReader(stdout)
	stderr, err := cmd.StderrPipe()

	if err != nil {
		m.lgr.Error().AnErr("failed creating command stderrPipe", err).Push()

		return
	}

	defer stderr.Close()

	stderrReader := bufio.NewReader(stderr)

	if err := cmd.Start(); err != nil {
		m.lgr.Error().AnErr("Failed starting command", err).Push()

		return
	}

	go handleReader(stdoutReader, command, m.lgr)
	go handleErrorReader(stderrReader, command, m.lgr)

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				m.lgr.Error().Int("Exit Status", status.ExitStatus())

				return
			}
		}

		return
	}
}

func nameReplace(cmd Command) string {
	if cmd.Format.CmdName == `` {
		return fmt.Sprintf(`%s %s`, cmd.getName(), strings.Join(cmd.Args, ` `))
	}

	tlpList := [2]string{`%CMD_NAME%`, `%CMD_ARGS%`}
	valueList := [2]string{cmd.getName(), strings.Join(cmd.Args, ` `)}

	result := cmd.Format.CmdName
	for idx, tpl := range tlpList {
		result = strings.ReplaceAll(result, tpl, valueList[idx])
	}

	return result
}

func handleReader(reader *bufio.Reader, cmd Command, log *reggol.Logger) error {
	chain := cmd.GetChain()
	chainName := strings.ToUpper(chain.Name)
	chainNameStyle := chain.Color
	div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(`>`)

	chainNameStyleText := chainNameStyle.Wrap(chainName) + ` ` + div
	cmdName := nameReplace(cmd)
	i := 0

	for {
		str, err := reader.ReadString('\n')
		if len(str) == 0 && err != nil {
			if err == io.EOF {
				break
			}

			log.Err(err).Push()

			return err
		}

		str = strings.TrimSuffix(str, "\n")

		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, i, div)
		log.Log().Blocks(chainNameStyleText, cmdNameStyled, str).Push()

		i++

		if err != nil {
			if err == io.EOF {
				break
			}

			log.Err(err).Push()

			return err
		}
	}

	return nil
}

func handleErrorReader(reader *bufio.Reader, cmd Command, log *reggol.Logger) error {
	chain := cmd.GetChain()
	chainName := strings.ToUpper(chain.Name)
	chainNameStyle := chain.Color
	chainNameStyleText := chainNameStyle.Wrap(chainName + ` >`)

	cmdName := nameReplace(cmd)

	for {
		str, err := reader.ReadString('\n')

		if len(str) == 0 && err != nil {
			if err == io.EOF {
				break
			}

			log.Err(err).Push()

			return err
		}

		str = strings.TrimSuffix(str, "\n")

		log.Err(errors.New(str)).Blocks(chainNameStyleText, cmdName).Push()

		if err != nil {
			if err == io.EOF {
				break
			}

			log.Err(err).Push()

			return err
		}
	}

	return nil
}

func (m *manager) RunParallel(ctx context.Context, chains []CommandChain) {
	var waitGroup sync.WaitGroup

	waitGroup.Add(len(chains))

	defer waitGroup.Wait()

	for _, chain := range chains {
		go func(ch CommandChain) {
			defer waitGroup.Done()

			m.RunChain(ctx, ch)
		}(chain)
	}
}

func (m *manager) RunChain(ctx context.Context, chain CommandChain) {
	for _, cmd := range chain.commands {
		if cmd.Pipe {
			m.RunWithPipe(ctx, cmd)
		} else {
			m.Run(ctx, cmd)
		}
	}
}
