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
	ConfigFilePath   string
	VersionRequested bool
}

// Option defines a functional option for configuring flag parsing.
type Option func(*flag.FlagSet)

// ParseFlags parses command line flags and returns the configuration.
// It accepts optional Option functions to customize the flag parsing behavior.
func ParseFlags(opts ...Option) (*Config, error) {
	fs := flag.NewFlagSet("parallel", flag.ContinueOnError)

	var cfg Config
	fs.StringVar(&cfg.ConfigFilePath, "f", defaultConfigPath, "Path to YAML configuration file")
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
		return nil, fmt.Errorf("parsing flags: %w", err)
	}

	// Validate the config
	if cfg.ConfigFilePath == "" {
		return nil, fmt.Errorf("config file path cannot be empty")
	}

	return &cfg, nil
}
