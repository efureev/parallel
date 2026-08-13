package config

import (
	"strings"
	"testing"
	"time"
)

// TestUnmarshal_TimeoutField — поле разбирается как длительность, а не как число.
func TestUnmarshal_TimeoutField(t *testing.T) {
	tests := []struct {
		name string
		line string
		want time.Duration
	}{
		{name: "секунды", line: "      timeout: 30s\n", want: 30 * time.Second},
		{name: "составная", line: "      timeout: 1m30s\n", want: 90 * time.Second},
		{name: "миллисекунды", line: "      timeout: 500ms\n", want: 500 * time.Millisecond},
		{name: "поля нет", line: "", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte("commands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n" + tt.line)

			cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got := cfg.Chains[0].Commands[0].Spec.Timeout; got != tt.want {
				t.Errorf("timeout = %s, ожидалось %s", got, tt.want)
			}
		})
	}
}

// TestUnmarshal_TimeoutWithoutUnit — голое число почти всегда означает секунды
// в голове автора, но не в разборе. Отказ обязан объяснять, чего не хватает:
// сообщение библиотеки про uint64 и time.Duration не объясняет ничего.
func TestUnmarshal_TimeoutWithoutUnit(t *testing.T) {
	raw := []byte("commands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n      timeout: 30\n")

	_, err := (YamlFileMarshaller{}).Unmarshal(raw)
	if err == nil {
		t.Fatal("число без единицы измерения принято без ошибки")
	}

	for _, want := range []string{"единицей измерения", "30s"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в сообщении нет %q:\n%v", want, err)
		}
	}
}

// TestBuild_TimeoutReachesCommand: docker-команда такая же команда, и предел
// обязан доезжать до обеих форм.
func TestBuild_TimeoutReachesCommand(t *testing.T) {
	spec := dockerSpec(nil)
	spec.Timeout = 15 * time.Second

	data := Data{Chains: []ChainConfig{{
		Name: "c",
		Commands: []NamedCommand{
			{Name: "regular", Spec: command{Cmd: []string{"echo"}, Timeout: 5 * time.Second}},
			{Name: "shell", Spec: command{Run: "echo hi", Timeout: 7 * time.Second}},
			{Name: "container", Spec: spec},
		},
	}}}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	want := map[string]time.Duration{
		"regular":   5 * time.Second,
		"shell":     7 * time.Second,
		"container": 15 * time.Second,
	}

	for _, cmd := range result.Chains[0].Commands() {
		if cmd.Timeout != want[cmd.Name] {
			t.Errorf("%s: timeout = %s, ожидалось %s", cmd.Name, cmd.Timeout, want[cmd.Name])
		}
	}
}
