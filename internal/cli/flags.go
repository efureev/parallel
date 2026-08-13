// Package cli разбирает аргументы командной строки, обрабатывает сигналы
// и связывает между собой остальные слои приложения.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/efureev/parallel/internal/ui"
)

// defaultLogLevel — уровень журналирования, если -log-level не задан.
const defaultLogLevel = "info"

var (
	// ErrEmptyConfigPath — флаг -f получил пустое значение.
	ErrEmptyConfigPath = errors.New("config file path cannot be empty")

	// ErrAdHocWithSelection — команды после `--` вместе с отбором цепочек.
	//
	// Отбор относится к конфигурации, а в режиме ad-hoc её нет: молча
	// проигнорировать одно из двух значило бы запустить не то, что просили.
	ErrAdHocWithSelection = errors.New("chain selection cannot be combined with commands after '--'")

	// ErrHelpRequested сообщает вызывающему, что пользователь попросил справку.
	//
	// Отдельная ошибка, а не общий отказ разбора: просьба показать справку —
	// не сбой, и завершаться из-за неё с ненулевым кодом нельзя.
	ErrHelpRequested = errors.New("help requested")
)

// Config хранит параметры запуска, разобранные из аргументов командной строки.
type Config struct {
	// ConfigFilePath пуст, если флаг -f не задан. Пустое значение означает
	// «искать конфигурацию самостоятельно», а не «взять файл в текущем
	// каталоге»: путь определяет слой config, поднимаясь по дереву каталогов.
	ConfigFilePath   string
	VersionRequested bool
	LogLevel         ui.Level

	// Chains — имена цепочек, названные позиционными аргументами; пусто
	// означает «все».
	Chains []string
	// Except — цепочки, которые надо исключить.
	Except []string
	// AdHoc — команды, переданные после `--`. Непусто означает запуск без
	// файла конфигурации.
	AdHoc []string

	// List печатает состав конфигурации и завершает работу.
	List bool
	// DryRun показывает разбор Flow и завершает работу, ничего не запуская.
	DryRun bool
	// NoColor принудительно отключает раскраску.
	NoColor bool

	// KeepGoing — отказ цепочки не останавливает соседние.
	KeepGoing bool
	// KeepGoingSet различает «флаг не передавали» и «передали -keep-going=false».
	// Без этого нельзя дать флагу перевесить ключ failFast из конфигурации.
	KeepGoingSet bool
}

// Option позволяет донастроить разбор флагов.
type Option func(*flag.FlagSet)

// usage печатает справку.
//
// Своя, а не автогенерируемая: стандартная показывала бы -v и -version двумя
// отдельными пунктами, не объясняла бы формат конфигурации и не давала примеров.
func usage(fs *flag.FlagSet) func() {
	return func() {
		out := fs.Output()

		fmt.Fprint(out, `parallel — run chains of console commands in parallel.

Usage:
  parallel [flags]

Flags:
  -f <path>          path to the YAML configuration file. If omitted, ".parallelrc.yaml"
                     (or ".parallelrc.yml") is looked up in the current directory and
                     every parent directory, the way git finds its config
  -except <names>    comma-separated chains to skip
  -list              list the chains defined in the configuration and exit
  -dry-run           show what would run and exit without starting anything
  -no-color          disable colored output (NO_COLOR is respected too)
  -keep-going        do not stop the other chains when one of them fails
                     (overrides failFast from the configuration file)
  -log-level <level> debug, info, warn or error (default "info")
  -v, --version      show version information and exit
  -h, --help         show this help and exit

Arguments:
  [chain...]         run only the named chains; the default is all of them
  -- <cmd>...        run the given shell commands in parallel, with no config file

Examples:
  parallel                              # find .parallelrc.yaml here or in a parent directory
  parallel api ui                       # run only these two chains
  parallel -except worker               # run everything but this one
  parallel -list                        # what does this configuration define?
  parallel -dry-run api                 # what exactly would 'parallel api' run?
  parallel -keep-going                  # report every failure, not just the first
  parallel -- 'go run ./cmd/api' 'yarn dev'   # no configuration file at all

Documentation: https://github.com/efureev/parallel
`)
	}
}

// bindFlags объявляет все флаги утилиты.
func bindFlags(fs *flag.FlagSet, cfg *Config, logLevel, except *string) {
	fs.StringVar(&cfg.ConfigFilePath, "f", "", "Path to YAML configuration file")
	fs.StringVar(except, "except", "", "Comma-separated chains to skip")
	fs.BoolVar(&cfg.List, "list", false, "List chains and exit")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Show what would run and exit")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "Disable colored output")
	fs.BoolVar(&cfg.KeepGoing, "keep-going", false, "Do not stop other chains when one fails")
	fs.StringVar(logLevel, "log-level", defaultLogLevel, "Log level: debug, info, warn, error")
	// Support both -v and -version flags.
	fs.BoolVar(&cfg.VersionRequested, "v", false, "Show version information and exit")
	fs.BoolVar(&cfg.VersionRequested, "version", false, "Show version information and exit")
}

// explicitlySet сообщает, присутствовал ли флаг в аргументах.
func explicitlySet(fs *flag.FlagSet, name string) bool {
	found := false

	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
}

// ParseFlags разбирает аргументы командной строки и возвращает конфигурацию.
//
// При запросе справки возвращает ErrHelpRequested: сама справка уже напечатана,
// а вызывающему остаётся завершиться с нулевым кодом.
func ParseFlags(opts ...Option) (*Config, error) {
	fs := flag.NewFlagSet("parallel", flag.ContinueOnError)
	fs.Usage = usage(fs)

	var (
		cfg      Config
		logLevel string
	)

	var except string

	bindFlags(fs, &cfg, &logLevel, &except)

	// Apply any custom options
	for _, opt := range opts {
		opt(fs)
	}

	// Разделитель `--` обрабатывается до разбора, а не после: flag.Parse
	// складывает и позиционные аргументы, и хвост после `--` в один и тот же
	// fs.Args(), а нам эти два случая нужно различать — первое это имена
	// цепочек, второе целые команды.
	head, adHoc := splitAtDoubleDash(os.Args[1:])
	cfg.AdHoc = adHoc

	// Preprocess args to support GNU-style --version alias.
	// The standard flag package does not recognize double-dash long booleans by default.
	args := make([]string, 0, len(head))
	for _, a := range head {
		if a == "--version" {
			a = "-version"
		}

		args = append(args, a)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, ErrHelpRequested
		}

		return nil, fmt.Errorf("parsing flags: %w", err)
	}

	// Пустой путь допустим только если -f не передавали вовсе: тогда файл ищется
	// автоматически. Явное `-f ""` — ошибка, и отличить один случай от другого
	// можно лишь по тому, посещал ли разборщик этот флаг.
	if explicitlySet(fs, "f") && cfg.ConfigFilePath == "" {
		return nil, ErrEmptyConfigPath
	}

	level, err := ui.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	cfg.LogLevel = level
	cfg.Chains = fs.Args()
	cfg.Except = splitList(except)
	cfg.KeepGoingSet = explicitlySet(fs, "keep-going")

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validate ловит взаимоисключающие сочетания аргументов.
func (c *Config) validate() error {
	if len(c.AdHoc) > 0 && (len(c.Chains) > 0 || len(c.Except) > 0) {
		return ErrAdHocWithSelection
	}

	return nil
}

// splitAtDoubleDash делит аргументы по первому литеральному `--`.
//
// Второе значение равно nil, когда разделителя не было вовсе: пустой, но
// непустой по длине срез означал бы «ad-hoc без команд», а это другой случай
// и другая ошибка.
func splitAtDoubleDash(args []string) (head, tail []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}

	return args, nil
}

// splitList разбирает список имён, перечисленных через запятую.
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")

	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}
