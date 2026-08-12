package config

import (
	"strconv"
	"strings"
	"testing"
)

const (
	benchChains           = 10
	benchCommandsPerChain = 10
)

// benchConfigYAML строит конфигурацию из chains цепочек по perChain команд в каждой.
func benchConfigYAML(chains, perChain int) []byte {
	var buf strings.Builder

	buf.WriteString("commands:\n")

	for c := range chains {
		buf.WriteString("  chain" + strconv.Itoa(c) + ":\n")

		for n := range perChain {
			idx := strconv.Itoa(n)
			buf.WriteString("    cmd" + idx + ":\n")
			buf.WriteString("      pipe: true\n")
			buf.WriteString("      cmd: [ 'echo', 'hello', '" + idx + "' ]\n")
			buf.WriteString("      dir: '/tmp'\n")
			buf.WriteString("      format: { cmdName: '%CMD_NAME% %CMD_ARGS%' }\n")
		}
	}

	return []byte(buf.String())
}

// BenchmarkParseConfig мерит разбор конфигурации через обход YAML-AST.
// Опорная точка для миграции на goccy/go-yaml.
func BenchmarkParseConfig(b *testing.B) {
	raw := benchConfigYAML(benchChains, benchCommandsPerChain)
	marshaller := YamlFileMarshaller{}

	b.ReportAllocs()

	for b.Loop() {
		cfg, err := marshaller.Unmarshal(raw)
		if err != nil {
			b.Fatalf("unmarshal: %v", err)
		}

		if len(cfg.Chains) != benchChains {
			b.Fatalf("expected %d chains, got %d", benchChains, len(cfg.Chains))
		}
	}
}
