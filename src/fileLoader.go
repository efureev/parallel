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
	Cmd     []string
	Docker  *dockerCommand
	Dir     string
	Pipe    bool
	Disable bool `yaml:"disable"`
	Format  format
}

// NamedCommand pairs a command spec with its config name, preserving order.
type NamedCommand struct {
	Name string
	Spec command
}

// ChainConfig is an ordered set of commands declared under a single chain name.
type ChainConfig struct {
	Name     string
	Commands []NamedCommand
}

// ConfigData is the ordered, in-memory representation of the parsed configuration.
// Order of chains and of commands inside each chain mirrors the YAML file exactly.
type ConfigData struct {
	Chains []ChainConfig
}

type FileLoader struct {
	marshaller FileMarshaller
}

func NewFileLoader(marshaller FileMarshaller) *FileLoader {
	return &FileLoader{marshaller: marshaller}
}

func (l *FileLoader) Load(filePath string) (ConfigData, error) {
	fileContent, err := l.loadFile(filePath)
	if err != nil {
		return ConfigData{}, fmt.Errorf("failed to load file: %w", err)
	}

	rawConfig, err := l.marshaller.Unmarshal(fileContent)
	if err != nil {
		return ConfigData{}, fmt.Errorf("failed to decode config file: %w", err)
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
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return ConfigData{}, err
	}

	return parseConfigData(&root)
}

// parseConfigData walks the YAML AST instead of decoding into maps so that the
// declaration order of chains and commands is preserved deterministically.
func parseConfigData(root *yaml.Node) (ConfigData, error) {
	var cfg ConfigData

	doc := root
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return cfg, nil
		}

		doc = doc.Content[0]
	}

	if doc.Kind != yaml.MappingNode {
		return cfg, nil
	}

	commandsNode := mappingValue(doc, "commands")
	if commandsNode == nil || commandsNode.Kind != yaml.MappingNode {
		return cfg, nil
	}

	for i := 0; i+1 < len(commandsNode.Content); i += 2 {
		chainName := commandsNode.Content[i].Value
		chainNode := commandsNode.Content[i+1]

		chain := ChainConfig{Name: chainName}

		if chainNode.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(chainNode.Content); j += 2 {
				cmdName := chainNode.Content[j].Value
				cmdNode := chainNode.Content[j+1]

				var spec command
				if err := cmdNode.Decode(&spec); err != nil {
					return ConfigData{}, fmt.Errorf("failed to decode command %q in chain %q: %w", cmdName, chainName, err)
				}

				chain.Commands = append(chain.Commands, NamedCommand{Name: cmdName, Spec: spec})
			}
		}

		cfg.Chains = append(cfg.Chains, chain)
	}

	return cfg, nil
}

// mappingValue returns the value node for the given key in a mapping node,
// or nil if the key is absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}

	return nil
}
