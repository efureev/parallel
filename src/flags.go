package parallel

import (
	"flag"
)

type FlagsConfig struct {
	ConfigFilePath string
}

func ParseFlag() FlagsConfig {
	var f string

	flag.StringVar(&f, "f", ".parallelrc.yaml", "YAML Config")
	flag.Parse()

	return FlagsConfig{
		ConfigFilePath: f,
	}
}
