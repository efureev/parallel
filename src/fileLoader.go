package parallel

import (
	"fmt"
	"os"

	"github.com/efureev/reggol"
	"gopkg.in/yaml.v3"
)

type FileMarshaller interface {
	Unmarshal(b []byte) (ConfigData, error)
}

type format struct {
	CmdName string `yaml:"cmdName"`
}

type dockerCommand struct {
	Image struct {
		Name string `yaml:"name"`
		Tag  string `yaml:"tag"`  // latest
		Pull string `yaml:"pull"` // 'always',
	} `yaml:"image"`
	RemoveAfterAll *bool    `yaml:"removeAfterAll"` // true
	Cmd            string   `yaml:"cmd"`            // run
	Ports          []string `yaml:"ports"`
}

type command struct {
	Cmd    []string
	Docker *dockerCommand
	Dir    string
	Pipe   bool
	Format format
}

type ConfigData = map[string]CommandChainData
type CommandChainData = map[string]map[string]command

type FileLoader struct {
	marshaller FileMarshaller
	lgr        *reggol.Logger
}

func NewFileLoader(marshaller FileMarshaller, lgr *reggol.Logger) *FileLoader {
	return &FileLoader{marshaller: marshaller, lgr: lgr}
}

func (l *FileLoader) Load(filePath string) (Flow, error) {
	defer l.lgr.Debug().Msg(`Config Parsed`)

	fileContent, err := l.loadFile(filePath)
	if err != nil {
		return Flow{}, fmt.Errorf("failed to load file: %w", err)
	}

	rawConfig, err := l.marshaller.Unmarshal(fileContent)
	if err != nil {
		return Flow{}, fmt.Errorf("failed to decode config file: %w", err)
	}

	flow := l.transformToStruct(rawConfig)

	return flow, nil
}

// Note: loadFile method should also be modified to return error instead of using Fatal:
func (l *FileLoader) loadFile(filePath string) ([]byte, error) {
	if filePath == `` {
		return nil, fmt.Errorf("missing config file path")
	}

	if !PathExists(filePath) {
		l.lgr.Fatal().Str(`file`, filePath).Msgf("config file not found: %s", filePath)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	l.lgr.Debug().Msg(`Config-File Loaded`)

	return fileContent, nil
}

func (l *FileLoader) transformToStruct(data ConfigData) Flow {
	commands, ok := data[`commands`]
	if !ok {
		l.lgr.Fatal().Str(`field`, `commands`).Msg(`Missing Config Field`)
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
				cmd = l.createDockerCommand(cmdName, cmdRaw)
			} else {
				cmd = l.createRegularCommand(cmdName, cmdRaw)
			}
			chain.Add(cmd)
		}
		flow.AddChain(chain)
	}
	return *flow
}

func (l *FileLoader) createDockerCommand(cmdName string, cmdRaw command) Command {
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

func (l *FileLoader) createRegularCommand(cmdName string, cmdRaw command) Command {
	return Command{
		Name:   cmdName,
		Cmd:    cmdRaw.Cmd[0],
		Args:   cmdRaw.Cmd[1:],
		Dir:    cmdRaw.Dir,
		Pipe:   cmdRaw.Pipe,
		Format: Format{CmdName: cmdRaw.Format.CmdName},
	}
}

type YamlFileMarshaller struct {
}

func (l YamlFileMarshaller) Unmarshal(b []byte) (ConfigData, error) {
	var c ConfigData
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return c, nil
}
