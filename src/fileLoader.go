package parallel

import (
	"os"

	"github.com/efureev/reggol"
	"gopkg.in/yaml.v3"
)

type FileMarshaller interface {
	Unmarshal(b []byte) (flowRaw, error)
}

type command struct {
	Cmd  []string
	Dir  string
	Pipe bool
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
		l.lgr.Panic().Msg(`missing config-file`)
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
			chain.Add(
				Command{
					Name: cmdName,
					Cmd:  cmdRaw.Cmd[0],
					Args: cmdRaw.Cmd[1:],
					Dir:  cmdRaw.Dir,
					Pipe: cmdRaw.Pipe,
				})
		}

		flow.AddChain(chain)
	}

	return *flow
}

func (l *FileLoader) loadFile(file string) []byte {
	if file == `` {
		l.lgr.Panic().Msg(`missing config-file`)
	}

	if !IsExistPath(file) {
		l.lgr.Panic().Str(`file`, file).Msg(`missing config-file`)
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
