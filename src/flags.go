package parallel

import (
	"flag"
	"fmt"
	"os"
)

// Default configuration values.
const (
	defaultConfigPath = ".parallelrc.yaml"
)

// Config holds the application configuration parameters parsed from command line flags.
type Config struct {
	ConfigFilePath string
}

// Option defines a functional option for configuring flag parsing.
type Option func(*flag.FlagSet)

// ParseFlags parses command line flags and returns the configuration.
// It accepts optional Option functions to customize the flag parsing behavior.
func ParseFlags(opts ...Option) (*Config, error) {
	fs := flag.NewFlagSet("parallel", flag.ContinueOnError)

	var cfg Config
	fs.StringVar(&cfg.ConfigFilePath, "f", defaultConfigPath, "Path to YAML configuration file")

	// Apply any custom options
	for _, opt := range opts {
		opt(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("parsing flags: %w", err)
	}

	// Validate the config
	if cfg.ConfigFilePath == "" {
		return nil, fmt.Errorf("config file path cannot be empty")
	}

	return &cfg, nil
}
