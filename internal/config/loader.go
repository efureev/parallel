// Package config отвечает за чтение YAML-конфигурации и её разбор
// в упорядоченное представление, из которого собирается доменный Flow.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/efureev/parallel/internal/flow"
)

// Верхнеуровневые ключи конфигурации.
const (
	commandsKey = "commands"
	failFastKey = "failFast"
)

// knownTopLevelFields — ключи, которые утилита понимает на верхнем уровне.
//
// Список нужен только для подсказки при опечатке. Неизвестный ключ здесь не
// ошибка, в отличие от полей команды: верхний уровень исторически принимал
// что угодно, и запрет сломал бы конфигурации с YAML-якорями (`_defaults: &d`),
// а схема заморожена с v1.0.0.
//
//nolint:gochecknoglobals // неизменяемый список, константой объявить нельзя
var knownTopLevelFields = []string{commandsKey, failFastKey}

// knownCommandFields — имена полей команды в том виде, в каком их пишут в YAML.
// Список нужен только для подсказки при опечатке; источник истины — yaml-теги
// структуры command ниже, и при добавлении поля его надо дописать сюда же.
//
//nolint:gochecknoglobals // неизменяемый список, массивом объявить нельзя
var knownCommandFields = []string{"cmd", "run", "docker", "dir", "pipe", "disable", "env", "format"}

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
	Cmd []string `yaml:"cmd"`
	// Run — та же команда одной строкой, разворачивается в вызов оболочки.
	// Сахар над cmd: [ 'sh', '-c', ... ], но именно так команды пишут везде,
	// и без него строку приходится разбивать руками.
	Run     string `yaml:"run"`
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

	// FailFast: nil означает, что ключа в файле не было и решение за вызывающим.
	// Указатель, а не bool, именно ради этого различия: false в файле и молчание
	// файла — разные вещи, потому что флаг командной строки сильнее только
	// второго.
	FailFast *bool

	// TopLevelHints — предупреждения о ключах верхнего уровня, похожих на
	// известные. Возвращаются данными, а не пишутся в лог: слой конфигурации
	// логгера не имеет, и заводить его ради двух строк незачем.
	TopLevelHints []string

	// BaseDir — каталог, относительно которого разрешаются относительные пути
	// в поле dir. Это каталог самого файла конфигурации, а не текущий каталог
	// процесса: конфигурация лежит рядом с проектом и коммитится вместе с ним,
	// поэтому должна работать откуда угодно, а не только из «правильного» места.
	BaseDir string
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

	rawConfig.BaseDir = baseDir(filePath)

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

// baseDir возвращает каталог файла конфигурации в абсолютном виде.
// Если абсолютный путь получить не удалось, остаётся относительный: это хуже,
// но лучше, чем потерять базу совсем.
func baseDir(configPath string) string {
	if abs, err := filepath.Abs(configPath); err == nil {
		return filepath.Dir(abs)
	}

	return filepath.Dir(configPath)
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

	cfg.TopLevelHints = topLevelHints(root)

	failFast, err := parseFailFast(root)
	if err != nil {
		return Data{}, err
	}

	cfg.FailFast = failFast

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
			if err := yaml.NodeToValue(cmdEntry.Value, &spec, yaml.Strict()); err != nil {
				return Data{}, decodeError(cmdName, chain.Name, err)
			}

			chain.Commands = append(chain.Commands, NamedCommand{Name: cmdName, Spec: spec})
		}

		cfg.Chains = append(cfg.Chains, chain)
	}

	return cfg, nil
}

// parseFailFast читает верхнеуровневый ключ failFast.
func parseFailFast(root []*ast.MappingValueNode) (*bool, error) {
	node := lookup(root, failFastKey)
	if node == nil {
		return nil, nil //nolint:nilnil // отсутствие ключа — не ошибка и не значение
	}

	var value bool
	if err := yaml.NodeToValue(node, &value, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrConfigDecode, failFastKey, err)
	}

	return &value, nil
}

// topLevelHints ищет ключи верхнего уровня, похожие на известные.
//
// Опечатка `failFats` иначе не сделала бы ничего и об этом не сказала — ровно
// тот класс ошибки, ради которого поля команды разбираются строго. Строгость
// здесь недоступна (см. knownTopLevelFields), поэтому остаётся предупреждение,
// и только для похожих: `x-common` и `_defaults` должны молчать.
func topLevelHints(root []*ast.MappingValueNode) []string {
	var hints []string

	for _, entry := range root {
		key := entry.Key.GetToken().Value
		if slices.Contains(knownTopLevelFields, key) {
			continue
		}

		if best := closestField(key, knownTopLevelFields); best != "" {
			hints = append(hints, fmt.Sprintf("unknown top-level key %q, possibly meant %q", key, best))
		}
	}

	return hints
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

// decodeError дополняет ошибку разбора именами команды и цепочки, а для
// неизвестного поля — подсказкой с ближайшим известным именем.
//
// Позицию не приписываем: goccy уже отдаёт её точнее — с номером строки,
// колонкой и фрагментом исходника. От нас требуется контекст, которого
// библиотека знать не может: в какой команде какой цепочки произошла ошибка.
func decodeError(cmdName, chainName string, err error) error {
	msg := fmt.Sprintf("command %q in chain %q", cmdName, chainName)

	if hint := unknownFieldHint(err); hint != "" {
		msg += ", " + hint
	}

	return fmt.Errorf("%s: %w", msg, err)
}

// unknownFieldRe вытаскивает имя поля из сообщения goccy про неизвестное поле.
var unknownFieldRe = regexp.MustCompile(`unknown field "([^"]+)"`)

// unknownFieldHint предлагает ближайшее известное поле для опечатки.
//
// Без подсказки строгий разбор ловит опечатку, но не помогает её исправить:
// «unknown field "pipeline"» оставляет пользователя гадать, как поле зовётся
// на самом деле.
func unknownFieldHint(err error) string {
	m := unknownFieldRe.FindStringSubmatch(err.Error())
	if m == nil {
		return ""
	}

	if best := closestField(m[1], knownCommandFields); best != "" {
		return fmt.Sprintf("возможно, имелось в виду %q", best)
	}

	return ""
}

// closestField ищет известное поле, ближайшее к введённому.
//
// Два правила вместо одного. Расстояние редактирования ловит перестановки и
// лишние буквы («diir» → «dir»), но бессильно против дописанного окончания:
// «pipeline» отстоит от «pipe» на четыре правки, а это ровно та опечатка,
// ради которой всё затевалось. Поэтому общий префикс считается совпадением
// сам по себе.
//
// Порог расстояния намеренно низкий: при трёх «xyz» оказывается одинаково
// близко к «env», «dir» и «cmd», и подсказка становится вредной.
func closestField(field string, known []string) string {
	const (
		maxDistance  = 2
		minPrefixLen = 3
	)

	lower := strings.ToLower(field)
	best, bestDist := "", maxDistance+1

	for _, name := range known {
		lowerName := strings.ToLower(name)

		if len(name) >= minPrefixLen && (strings.HasPrefix(lower, lowerName) || strings.HasPrefix(lowerName, lower)) {
			return name
		}

		if d := editDistance(lower, lowerName); d < bestDist {
			best, bestDist = name, d
		}
	}

	if bestDist > maxDistance {
		return ""
	}

	return best
}

// editDistance — расстояние Левенштейна между строками.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i

		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}

		prev, curr = curr, prev
	}

	return prev[len(b)]
}
