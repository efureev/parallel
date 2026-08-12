package config

import (
	"github.com/efureev/reggol"

	"github.com/efureev/parallel/internal/flow"
)

// FlowBuilder собирает доменный Flow из загруженной конфигурации.
type FlowBuilder struct {
	lgr *reggol.Logger
}

func NewFlowBuilder(lgr *reggol.Logger) *FlowBuilder {
	return &FlowBuilder{lgr: lgr}
}

// Build преобразует Data в доменную структуру Flow.
// Порядок цепочек и команд внутри них сохраняется ровно таким, каким он задан в конфигурации.
func (b *FlowBuilder) Build(data Data) flow.Flow {
	if len(data.Chains) == 0 {
		b.lgr.Error().Str(`field`, `commands`).Msg(`Missing Config Field`)

		return flow.Flow{}
	}

	colorList := flow.GenColors(true)

	var currentColor reggol.TextStyle

	result := &flow.Flow{}

	for _, chainCfg := range data.Chains {
		currentColor, colorList = colorList[0], colorList[1:]
		if len(colorList) == 0 {
			colorList = flow.GenColors(true)
		}

		chain := &flow.CommandChain{
			Name:  chainCfg.Name,
			Color: currentColor,
		}

		for _, namedCmd := range chainCfg.Commands {
			var cmd flow.Command
			if namedCmd.Spec.Docker != nil {
				cmd = b.createDockerCommand(namedCmd.Name, namedCmd.Spec)
			} else {
				cmd = b.createRegularCommand(namedCmd.Name, namedCmd.Spec)
			}

			chain.Add(cmd)
		}

		result.AddChain(chain)
	}

	return *result
}

func (b *FlowBuilder) createDockerCommand(cmdName string, cmdRaw command) flow.Command {
	dockerCmd := cmdRaw.Docker.Cmd
	if dockerCmd == `` {
		dockerCmd = `run`
	}

	args := []string{dockerCmd, `--name`, cmdName}

	if cmdRaw.Docker.RemoveAfterAll == nil {
		args = append(args, `--rm`)
	}

	if cmdRaw.Docker.Image.Pull != `` {
		args = append(args, `--pull`, cmdRaw.Docker.Image.Pull)
	}

	for _, port := range cmdRaw.Docker.Ports {
		args = append(args, `-p`, port)
	}

	imageTag := cmdRaw.Docker.Image.Tag
	if imageTag == `` {
		imageTag = `latest`
	}

	imageName := cmdRaw.Docker.Image.Name + `:` + imageTag
	args = append(args, imageName)

	return flow.Command{
		Name:    cmdName,
		Cmd:     `docker`,
		Args:    args,
		Dir:     cmdRaw.Dir,
		Pipe:    true,
		Disable: cmdRaw.Disable,
		Format:  flow.Format{CmdName: cmdRaw.Format.CmdName},
	}
}

func (b *FlowBuilder) createRegularCommand(cmdName string, cmdRaw command) flow.Command {
	var (
		cmdStr string
		args   []string
	)

	if len(cmdRaw.Cmd) > 0 {
		cmdStr = cmdRaw.Cmd[0]
		if len(cmdRaw.Cmd) > 1 {
			args = cmdRaw.Cmd[1:]
		}
	}

	return flow.Command{
		Name:    cmdName,
		Cmd:     cmdStr,
		Args:    args,
		Dir:     cmdRaw.Dir,
		Pipe:    cmdRaw.Pipe,
		Disable: cmdRaw.Disable,
		Format:  flow.Format{CmdName: cmdRaw.Format.CmdName},
	}
}
