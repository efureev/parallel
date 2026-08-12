// Package cli разбирает аргументы командной строки, обрабатывает сигналы
// и связывает между собой остальные слои приложения.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/efureev/parallel/internal/ui"
)

// Значения по умолчанию.
const (
	defaultConfigPath = ".parallelrc.yaml"
	defaultLogLevel   = "info"
)

var (
	// ErrEmptyConfigPath — флаг -f получил пустое значение.
	ErrEmptyConfigPath = errors.New("config file path cannot be empty")

	// ErrHelpRequested сообщает вызывающему, что пользователь попросил справку.
	//
	// Отдельная ошибка, а не общий отказ разбора: просьба показать справку —
	// не сбой, и завершаться из-за неё с ненулевым кодом нельзя.
	ErrHelpRequested = errors.New("help requested")
)

// Config хранит параметры запуска, разобранные из аргументов командной строки.
type Config struct {
	ConfigFilePath   string
	VersionRequested bool
	LogLevel         ui.Level
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
  -f <path>          path to the YAML configuration file (default ".parallelrc.yaml")
  -log-level <level> debug, info, warn or error (default "info")
  -v, --version      show version information and exit
  -h, --help         show this help and exit

Examples:
  parallel                            # use .parallelrc.yaml from the current directory
  parallel -f examples/basic.yaml     # use an explicit configuration file
  parallel -log-level debug           # show what the tool is doing internally

Documentation: https://github.com/efureev/parallel
`)
	}
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

	fs.StringVar(&cfg.ConfigFilePath, "f", defaultConfigPath, "Path to YAML configuration file")
	fs.StringVar(&logLevel, "log-level", defaultLogLevel, "Log level: debug, info, warn, error")
	// Support both -v and -version flags
	fs.BoolVar(&cfg.VersionRequested, "v", false, "Show version information and exit")
	fs.BoolVar(&cfg.VersionRequested, "version", false, "Show version information and exit")

	// Apply any custom options
	for _, opt := range opts {
		opt(fs)
	}

	// Preprocess os.Args to support GNU-style --version alias
	// The standard flag package does not recognize double-dash long booleans by default.
	args := make([]string, 0, len(os.Args)-1)
	for _, a := range os.Args[1:] {
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

	// Validate the config
	if cfg.ConfigFilePath == "" {
		return nil, ErrEmptyConfigPath
	}

	level, err := ui.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	cfg.LogLevel = level

	return &cfg, nil
}
