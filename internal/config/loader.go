// Package config отвечает за чтение YAML-конфигурации и её разбор
// в упорядоченное представление, из которого собирается доменный Flow.
package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/efureev/parallel/internal/flow"
)

// commandsKey — верхнеуровневый ключ конфигурации.
const commandsKey = "commands"

// FileMarshaller разбирает содержимое файла конфигурации.
type FileMarshaller interface {
	Unmarshal(b []byte) (Data, error)
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
	Cmd     []string `yaml:"cmd"`
	Docker  *dockerCommand
	Dir     string
	Pipe    bool
	Disable bool              `yaml:"disable"`
	Env     map[string]string `yaml:"env"`
	Format  format
}

// NamedCommand связывает спецификацию команды с её именем, сохраняя порядок.
type NamedCommand struct {
	Name string
	Spec command
}

// ChainConfig — упорядоченный набор команд под одним именем цепочки.
type ChainConfig struct {
	Name     string
	Commands []NamedCommand
}

// Data — упорядоченное представление разобранной конфигурации.
// Порядок цепочек и команд внутри них в точности повторяет YAML-файл.
type Data struct {
	Chains []ChainConfig
}

// FileLoader читает файл конфигурации и передаёт его разборщику.
type FileLoader struct {
	marshaller FileMarshaller
}

func NewFileLoader(marshaller FileMarshaller) *FileLoader {
	return &FileLoader{marshaller: marshaller}
}

func (l *FileLoader) Load(filePath string) (Data, error) {
	fileContent, err := l.loadFile(filePath)
	if err != nil {
		return Data{}, err
	}

	rawConfig, err := l.marshaller.Unmarshal(fileContent)
	if err != nil {
		return Data{}, fmt.Errorf("%w %s: %w", ErrConfigDecode, filePath, err)
	}

	return rawConfig, nil
}

func (l *FileLoader) loadFile(filePath string) ([]byte, error) {
	if filePath == `` {
		return nil, ErrEmptyConfigPath
	}

	if !flow.PathExists(filePath) {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, filePath)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrConfigRead, filePath, err)
	}

	return fileContent, nil
}

// YamlFileMarshaller разбирает конфигурацию из YAML.
type YamlFileMarshaller struct {
}

func (l YamlFileMarshaller) Unmarshal(b []byte) (Data, error) {
	file, err := parser.ParseBytes(b, 0)
	if err != nil {
		// Ошибка разбора от goccy уже содержит строку, колонку и фрагмент исходника.
		return Data{}, err
	}

	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return Data{}, nil
	}

	return parseData(file.Docs[0].Body)
}

// parseData обходит YAML-AST вместо декодирования в мапы, чтобы порядок
// объявления цепочек и команд сохранялся детерминированно.
//
// Это требование, а не вкус: порядок виден пользователю в предпросмотре Flow
// и в раскраске цепочек, а обход Go-мапы рандомизирован. Инвариант закреплён
// тестом TestYamlMarshaller_PreservesOrder.
func parseData(body ast.Node) (Data, error) {
	var cfg Data

	root := mappingValues(body)
	if root == nil {
		return cfg, nil
	}

	commandsNode := lookup(root, commandsKey)
	if commandsNode == nil {
		return cfg, nil
	}

	chains := mappingValues(commandsNode)
	if chains == nil {
		return cfg, nil
	}

	for _, chainEntry := range chains {
		chain := ChainConfig{Name: chainEntry.Key.GetToken().Value}

		for _, cmdEntry := range mappingValues(chainEntry.Value) {
			cmdName := cmdEntry.Key.GetToken().Value

			var spec command
			if err := yaml.NodeToValue(cmdEntry.Value, &spec); err != nil {
				return Data{}, decodeError(cmdEntry.Value, cmdName, chain.Name, err)
			}

			chain.Commands = append(chain.Commands, NamedCommand{Name: cmdName, Spec: spec})
		}

		cfg.Chains = append(cfg.Chains, chain)
	}

	return cfg, nil
}

// mappingValues приводит узел к списку пар «ключ-значение» в исходном порядке.
//
// goccy представляет отображение с единственным ключом как MappingValueNode,
// а не как MappingNode — оба случая обрабатываются здесь, иначе конфигурация
// из одной цепочки разбиралась бы иначе, чем из нескольких.
func mappingValues(n ast.Node) []*ast.MappingValueNode {
	switch node := n.(type) {
	case *ast.MappingNode:
		return node.Values
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{node}
	default:
		return nil
	}
}

// lookup возвращает значение по ключу либо nil, если ключа нет.
func lookup(values []*ast.MappingValueNode, key string) ast.Node {
	for _, v := range values {
		if v.Key.GetToken().Value == key {
			return v.Value
		}
	}

	return nil
}

// decodeError дополняет ошибку разбора именами команды и цепочки.
//
// Позицию не приписываем: goccy уже отдаёт её точнее — с номером строки,
// колонкой и фрагментом исходника, — а вторая пара чисел рядом только сбивала бы
// с толку. От нас требуется контекст, которого библиотека знать не может:
// в какой команде какой цепочки произошла ошибка.
func decodeError(_ ast.Node, cmdName, chainName string, err error) error {
	return fmt.Errorf("command %q in chain %q: %w", cmdName, chainName, err)
}
