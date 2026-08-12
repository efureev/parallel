package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadErrorsAreSentinels фиксирует, что ошибки слоя конфигурации
// сравнимы через errors.Is: это самые частые ошибки в эксплуатации,
// и вызывающий код должен уметь их различать.
func TestLoadErrorsAreSentinels(t *testing.T) {
	loader := NewFileLoader(YamlFileMarshaller{})

	t.Run("пустой путь", func(t *testing.T) {
		_, err := loader.Load("")
		if !errors.Is(err, ErrEmptyConfigPath) {
			t.Fatalf("expected ErrEmptyConfigPath, got %v", err)
		}
	})

	t.Run("файла нет", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.yaml")

		_, err := loader.Load(missing)
		if !errors.Is(err, ErrConfigNotFound) {
			t.Fatalf("expected ErrConfigNotFound, got %v", err)
		}

		// Путь обязан остаться в тексте: без него сообщение бесполезно.
		if got := err.Error(); !strings.Contains(got, missing) {
			t.Errorf("error must mention the path %q, got %q", missing, got)
		}
	})

	t.Run("битый YAML", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "broken.yaml")
		if err := os.WriteFile(path, []byte("commands:\n  a:\n   - ]["), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, err := loader.Load(path)
		if !errors.Is(err, ErrConfigDecode) {
			t.Fatalf("expected ErrConfigDecode, got %v", err)
		}
	})
}
