package parallel

import (
	"strings"
	"testing"
)

func TestFlowBuilder_BuildMissingCommands(t *testing.T) {
	b := NewFlowBuilder(Logger())

	flow := b.Build(ConfigData{})

	if len(flow.Chains) != 0 {
		t.Fatalf("expected 0 chains when 'commands' missing, got %d", len(flow.Chains))
	}
}

func TestFlowBuilder_BuildRegularAndDocker(t *testing.T) {
	b := NewFlowBuilder(Logger())

	data := ConfigData{
		Chains: []ChainConfig{
			{
				Name: "c1",
				Commands: []NamedCommand{
					{Name: "hello", Spec: command{Cmd: []string{"echo", "hi"}}},
				},
			},
			{
				Name: "dock",
				Commands: []NamedCommand{
					{Name: "ng", Spec: command{Docker: &dockerCommand{Image: struct {
						Name string `yaml:"name"`
						Tag  string `yaml:"tag"`
						Pull string `yaml:"pull"`
					}{Name: "nginx"}, Ports: []string{"8080:80"}}}},
				},
			},
		},
	}

	flow := b.Build(data)

	if len(flow.Chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(flow.Chains))
	}

	var foundEcho, foundDocker bool

	for _, ch := range flow.Chains {
		for _, c := range ch.commands {
			if c.Cmd == "echo" && len(c.Args) == 1 && c.Args[0] == "hi" {
				foundEcho = true
			}

			if c.Cmd == "docker" {
				foundDocker = true
				// Validate essential docker args using simple substring checks to reduce complexity.
				argsStr := strings.Join(c.Args, " ")
				hasRun := strings.Contains(argsStr, "run")
				hasRm := strings.Contains(argsStr, "--rm")
				hasPort := strings.Contains(argsStr, "-p")
				hasImage := strings.Contains(argsStr, "nginx:latest")

				if !(hasRun && hasRm && hasPort && hasImage) {
					t.Fatalf("docker args missing expected flags: %s", argsStr)
				}
			}
		}
	}

	if !foundEcho || !foundDocker {
		t.Fatalf("expected to find both echo and docker commands, echo=%v docker=%v", foundEcho, foundDocker)
	}
}

func TestFlowBuilder_DisablePropagationAndDefault(t *testing.T) {
	b := NewFlowBuilder(Logger())

	data := ConfigData{
		Chains: []ChainConfig{
			{
				Name: "c1",
				Commands: []NamedCommand{
					{Name: "enabled", Spec: command{Cmd: []string{"echo", "ok"}}},
					{Name: "disabled", Spec: command{Cmd: []string{"echo", "no"}, Disable: true}},
				},
			},
			{
				Name: "dock",
				Commands: []NamedCommand{
					{Name: "ng", Spec: command{Docker: &dockerCommand{Image: struct {
						Name string `yaml:"name"`
						Tag  string `yaml:"tag"`
						Pull string `yaml:"pull"`
					}{Name: "nginx"}}, Disable: true}},
				},
			},
		},
	}

	flow := b.Build(data)

	// helper map by command Name
	states := map[string]bool{}
	for _, ch := range flow.Chains {
		for _, c := range ch.commands {
			states[c.Name] = c.Disable
		}
	}

	if got := states["enabled"]; got {
		t.Fatalf("expected enabled command to have Disable=false, got true")
	}

	if got := states["disabled"]; !got {
		t.Fatalf("expected disabled command to have Disable=true, got false")
	}

	if got := states["ng"]; !got {
		t.Fatalf("expected docker command 'ng' to have Disable=true, got false")
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
	for attempt := 0; attempt < 20; attempt++ {
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
