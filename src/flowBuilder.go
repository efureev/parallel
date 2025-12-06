package parallel

import "github.com/efureev/reggol"

// FlowBuilder отвечает за построение структуры Flow на основе загруженных данных конфигурации.
type FlowBuilder struct {
	lgr *reggol.Logger
}

func NewFlowBuilder(lgr *reggol.Logger) *FlowBuilder {
	return &FlowBuilder{lgr: lgr}
}

// Build преобразует ConfigData в доменную структуру Flow.
func (b *FlowBuilder) Build(data ConfigData) Flow {
	commands, ok := data[`commands`]
	if !ok {
		b.lgr.Error().Str(`field`, `commands`).Msg(`Missing Config Field`)

		return Flow{}
	}

	colorList := GenColors(true)

	var currentColor reggol.TextStyle

	flow := &Flow{}

	for chainName, chainRaw := range commands {
		currentColor, colorList = colorList[0], colorList[1:]
		if len(colorList) == 0 {
			colorList = GenColors(true)
		}

		chain := CommandChain{
			Name:  chainName,
			Color: currentColor,
		}

		for cmdName, cmdRaw := range chainRaw {
			var cmd Command
			if cmdRaw.Docker != nil {
				cmd = b.createDockerCommand(cmdName, cmdRaw)
			} else {
				cmd = b.createRegularCommand(cmdName, cmdRaw)
			}

			chain.Add(cmd)
		}

		flow.AddChain(chain)
	}

	return *flow
}

func (b *FlowBuilder) createDockerCommand(cmdName string, cmdRaw command) Command {
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

	return Command{
		Name:   cmdName,
		Cmd:    `docker`,
		Args:   args,
		Dir:    cmdRaw.Dir,
		Pipe:   true,
		Format: Format{CmdName: cmdRaw.Format.CmdName},
	}
}

func (b *FlowBuilder) createRegularCommand(cmdName string, cmdRaw command) Command {
	return Command{
		Name:   cmdName,
		Cmd:    cmdRaw.Cmd[0],
		Args:   cmdRaw.Cmd[1:],
		Dir:    cmdRaw.Dir,
		Pipe:   cmdRaw.Pipe,
		Format: Format{CmdName: cmdRaw.Format.CmdName},
	}
}
