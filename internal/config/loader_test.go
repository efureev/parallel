package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileLoader_LoadSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flow.yaml")

	content := []byte(`
commands:
  build:
    step:
      cmd: ['echo', 'hi']
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	loader := NewFileLoader(YamlFileMarshaller{})

	cfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if len(cfg.Chains) != 1 || cfg.Chains[0].Name != "build" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestFileLoader_LoadErrors(t *testing.T) {
	loader := NewFileLoader(YamlFileMarshaller{})

	if _, err := loader.Load(""); err == nil {
		t.Fatalf("expected error for empty path")
	}

	if _, err := loader.Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

// TestYamlMarshaller_PreservesOrder проверяет, что порядок цепочек и команд
// сохраняется ровно таким, каким он задан в YAML, и стабилен между запусками.
func TestYamlMarshaller_PreservesOrder(t *testing.T) {
	yamlCfg := []byte(`
commands:
  zeta:
    migrate:
      cmd: ['echo', 'migrate']
    serve:
      pipe: true
      cmd: ['echo', 'serve']
    health:
      pipe: true
      cmd: ['echo', 'health']
  alpha:
    one:
      cmd: ['echo', '1']
    two:
      cmd: ['echo', '2']
  beta:
    only:
      cmd: ['echo', 'only']
`)

	wantChains := []string{"zeta", "alpha", "beta"}
	wantZeta := []string{"migrate", "serve", "health"}

	// Повторяем парсинг несколько раз: результат обязан быть детерминированным.
	for attempt := range 20 {
		cfg, err := YamlFileMarshaller{}.Unmarshal(yamlCfg)
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if len(cfg.Chains) != len(wantChains) {
			t.Fatalf("expected %d chains, got %d", len(wantChains), len(cfg.Chains))
		}

		for i, want := range wantChains {
			if cfg.Chains[i].Name != want {
				t.Fatalf("attempt %d: chain[%d] = %q, want %q", attempt, i, cfg.Chains[i].Name, want)
			}
		}

		for i, want := range wantZeta {
			if got := cfg.Chains[0].Commands[i].Name; got != want {
				t.Fatalf("attempt %d: zeta command[%d] = %q, want %q", attempt, i, got, want)
			}
		}
	}
}
