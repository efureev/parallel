package config

import (
	"errors"
	"strings"
	"testing"
)

// TestAdHoc_ChainPerCommand — цепочка есть единица параллелизма, а просят
// именно параллельный запуск: сложив команды в одну цепочку, мы получили бы
// последовательный.
func TestAdHoc_ChainPerCommand(t *testing.T) {
	result, err := AdHoc([]string{"go run ./cmd/api", "yarn dev"})
	if err != nil {
		t.Fatalf("adhoc: %v", err)
	}

	if len(result.Chains) != 2 {
		t.Fatalf("цепочек %d, ожидалось 2", len(result.Chains))
	}

	if err := result.Validate(); err != nil {
		t.Errorf("собранный Flow невалиден: %v", err)
	}

	for _, chain := range result.Chains {
		cmd := chain.Commands()[0]
		if !cmd.Pipe {
			t.Errorf("цепочка %q не потоковая: запуск из командной строки — это наблюдение за выводом", chain.Name)
		}
	}
}

func TestAdHoc_Names(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{name: "первое слово", lines: []string{"go run ./cmd/api", "yarn dev"}, want: []string{"go", "yarn"}},
		{name: "одинаковые различаются", lines: []string{"echo a", "echo b"}, want: []string{"echo", "echo-2"}},
		{name: "путь сокращается", lines: []string{"/usr/local/bin/php -S localhost:80"}, want: []string{"php"}},
		{name: "присваивание пропускается", lines: []string{"FOO=bar node index.js"}, want: []string{"node"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AdHoc(tt.lines)
			if err != nil {
				t.Fatalf("adhoc: %v", err)
			}

			if got := strings.Join(result.Names(), ","); got != strings.Join(tt.want, ",") {
				t.Errorf("имена = %s, ожидались %s", got, strings.Join(tt.want, ","))
			}
		})
	}
}

// TestAdHoc_UsesShell: ради пайпов и && строку и передают одной строкой.
func TestAdHoc_UsesShell(t *testing.T) {
	result, err := AdHoc([]string{"echo hi && echo bye"})
	if err != nil {
		t.Fatalf("adhoc: %v", err)
	}

	cmd := result.Chains[0].Commands()[0]
	if len(cmd.Args) != 2 || cmd.Args[1] != "echo hi && echo bye" {
		t.Errorf("команда не ушла в оболочку целиком: %v %v", cmd.Cmd, cmd.Args)
	}
}

func TestAdHoc_Empty(t *testing.T) {
	for _, tt := range []struct {
		name  string
		lines []string
	}{
		{name: "совсем ничего", lines: nil},
		{name: "пустая строка", lines: []string{"echo ok", "   "}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := AdHoc(tt.lines); !errors.Is(err, ErrNoAdHocCommands) {
				t.Errorf("ожидалась ErrNoAdHocCommands, получено %v", err)
			}
		})
	}
}
