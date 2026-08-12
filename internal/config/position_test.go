package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigErrorHasPosition требует, чтобы ошибка разбора указывала место в файле.
// Без него сообщение сводится к именам команды и цепочки, и на конфигурации
// в полторы сотни строк искать приходится глазами.
func TestConfigErrorHasPosition(t *testing.T) {
	tests := []struct {
		name    string
		content string
		// wantParts — то, без чего сообщение бесполезно.
		wantParts []string
	}{
		{
			name: "неверный тип поля",
			content: "commands:\n" +
				"  api:\n" +
				"    serve:\n" +
				"      pipe: true\n" +
				"      cmd: [ 'echo', 'ok' ]\n" +
				"    broken:\n" +
				"      pipe: 'not-a-bool'\n" +
				"      cmd: [ 'echo' ]\n",
			// 7 — строка с битым значением; имена команды и цепочки даёт наш слой.
			wantParts: []string{"7:", "broken", "api"},
		},
		{
			name: "синтаксическая ошибка",
			content: "commands:\n" +
				"  api:\n" +
				"    serve:\n" +
				"      cmd: [ 'echo', 'ok'\n",
			wantParts: []string{"4:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "flow.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, err := NewFileLoader(YamlFileMarshaller{}).Load(path)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if !errors.Is(err, ErrConfigDecode) {
				t.Errorf("expected ErrConfigDecode, got %v", err)
			}

			msg := err.Error()

			// Имя файла: без него непонятно, какой из конфигов сломан.
			if !strings.Contains(msg, path) {
				t.Errorf("сообщение не содержит путь %q:\n%s", path, msg)
			}

			for _, want := range tt.wantParts {
				if !strings.Contains(msg, want) {
					t.Errorf("сообщение не содержит %q:\n%s", want, msg)
				}
			}

			// Фрагмент исходника с указателем — то, ради чего затевалась миграция.
			if !strings.Contains(msg, "|") || !strings.Contains(msg, ">") {
				t.Errorf("сообщение не содержит фрагмент исходника:\n%s", msg)
			}
		})
	}
}
