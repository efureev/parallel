package parallel

import (
	"os"

	"github.com/efureev/reggol"
	"gopkg.in/yaml.v3"
)

type FileMarshaller interface {
	Unmarshal(b []byte) (flowRaw, error)
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
	RemoveAfterAll bool     `yaml:"removeAfterAll"` // true
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

type flowRaw = map[string]commandChain

type commandChain = map[string]map[string]command

type FileLoader struct {
	marshaller FileMarshaller
	lgr        *reggol.Logger
}

func NewFileLoader(marshaller FileMarshaller, lgr *reggol.Logger) *FileLoader {
	return &FileLoader{marshaller: marshaller, lgr: lgr}
}

func (l *FileLoader) Load(file string) Flow {
	c, err := l.marshaller.Unmarshal(l.loadFile(file))
	if err != nil {
		l.lgr.Fatal().AnErr(`unable to decode config-file`, err).Push()
	}

	r := l.transformToStruct(c)

	defer l.lgr.Debug().Msg(`Config Parsed`)

	return r
}

func (l *FileLoader) transformToStruct(data flowRaw) Flow {
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
				dckrCmd := cmdRaw.Docker.Cmd
				if dckrCmd == `` {
					dckrCmd = `run`
				}

				args := []string{dckrCmd, `--name`, cmdName}

				if cmdRaw.Docker.RemoveAfterAll {
					args = append(args, `--rm`)
				}

				if cmdRaw.Docker.Image.Pull != `` {
					args = append(args, `--pull`, cmdRaw.Docker.Image.Pull)
				}

				for _, port := range cmdRaw.Docker.Ports {
					args = append(args, `-p`, port)
				}

				imgName := cmdRaw.Docker.Image.Name

				if cmdRaw.Docker.Image.Tag == `` {
					cmdRaw.Docker.Image.Tag = `latest`
				}

				args = append(args, imgName+`:`+cmdRaw.Docker.Image.Tag)

				cmd = Command{
					Name:   cmdName,
					Cmd:    `docker`,
					Args:   args,
					Dir:    cmdRaw.Dir,
					Pipe:   true,
					Format: Format{CmdName: cmdRaw.Format.CmdName},
				}
			} else {
				cmd = Command{
					Name:   cmdName,
					Cmd:    cmdRaw.Cmd[0],
					Args:   cmdRaw.Cmd[1:],
					Dir:    cmdRaw.Dir,
					Pipe:   cmdRaw.Pipe,
					Format: Format{CmdName: cmdRaw.Format.CmdName},
				}
			}

			chain.Add(cmd)
		}

		flow.AddChain(chain)
	}

	return *flow
}

func (l *FileLoader) loadFile(file string) []byte {
	if file == `` {
		l.lgr.Fatal().Msg(`missing a path of the config-file`)
	}

	if !IsExistPath(file) {
		l.lgr.Fatal().Str(`file`, file).Msg(`missing a config-file`)
	}

	f, err := os.ReadFile(file)

	if err != nil {
		l.lgr.Fatal().Err(err).Str(`file`, file).Push()
	}

	l.lgr.Debug().Msg(`Config-File Loaded`)

	return f
}

type YamlFileMarshaller struct {
}

func (l YamlFileMarshaller) Unmarshal(b []byte) (flowRaw, error) {
	var c flowRaw

	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	return c, nil
}
