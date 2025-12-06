package parallel

import (
	"fmt"
	"os"

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
}

func NewFileLoader(marshaller FileMarshaller) *FileLoader {
	return &FileLoader{marshaller: marshaller}
}

func (l *FileLoader) Load(filePath string) (ConfigData, error) {
	fileContent, err := l.loadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load file: %w", err)
	}

	rawConfig, err := l.marshaller.Unmarshal(fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return rawConfig, nil
}

func (l *FileLoader) loadFile(filePath string) ([]byte, error) {
	if filePath == `` {
		return nil, fmt.Errorf("missing config file path")
	}

	if !PathExists(filePath) {
		return nil, fmt.Errorf("config file not found: %s", filePath)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	return fileContent, nil
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
