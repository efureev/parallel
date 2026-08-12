package config

import (
	"errors"
	"strings"
	"testing"
)

// dockerImage собирает анонимную структуру образа так, как её объявляет dockerCommand.
func dockerImage(name string) struct {
	Name string `yaml:"name"`
	Tag  string `yaml:"tag"`
	Pull string `yaml:"pull"`
} {
	return struct {
		Name string `yaml:"name"`
		Tag  string `yaml:"tag"`
		Pull string `yaml:"pull"`
	}{Name: name}
}

func TestFlowBuilder_BuildMissingCommands(t *testing.T) {
	b := NewFlowBuilder()

	result, err := b.Build(Data{})

	if !errors.Is(err, ErrMissingCommands) {
		t.Fatalf("expected ErrMissingCommands, got %v", err)
	}

	if len(result.Chains) != 0 {
		t.Fatalf("expected 0 chains when 'commands' missing, got %d", len(result.Chains))
	}
}

func TestFlowBuilder_BuildRegularAndDocker(t *testing.T) {
	b := NewFlowBuilder()

	data := Data{
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
					{Name: "ng", Spec: command{Docker: &dockerCommand{
						Image: dockerImage("nginx"),
						Ports: []string{"8080:80"},
					}}},
				},
			},
		},
	}

	result, err := b.Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(result.Chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(result.Chains))
	}

	var foundEcho, foundDocker bool

	for _, ch := range result.Chains {
		for _, c := range ch.Commands() {
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

				if !hasRun || !hasRm || !hasPort || !hasImage {
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
	b := NewFlowBuilder()

	data := Data{
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
					{Name: "ng", Spec: command{Docker: &dockerCommand{Image: dockerImage("nginx")}, Disable: true}},
				},
			},
		},
	}

	result, err := b.Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	states := map[string]bool{}

	for _, ch := range result.Chains {
		for _, c := range ch.Commands() {
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

// TestFlowBuilder_PreservesOrder заменяет прежний тест на обратный указатель команды:
// после снятия связи Command→Chain гарантировать нужно уже не тождество родителя,
// а то, что команды разложены по своим цепочкам в исходном порядке.
func TestFlowBuilder_PreservesOrder(t *testing.T) {
	b := NewFlowBuilder()

	data := Data{
		Chains: []ChainConfig{
			{
				Name: "first",
				Commands: []NamedCommand{
					{Name: "a", Spec: command{Cmd: []string{"echo", "a"}}},
					{Name: "b", Spec: command{Cmd: []string{"echo", "b"}}},
				},
			},
			{
				Name: "second",
				Commands: []NamedCommand{
					{Name: "c", Spec: command{Cmd: []string{"echo", "c"}}},
				},
			},
		},
	}

	result, err := b.Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(result.Chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(result.Chains))
	}

	want := map[string][]string{
		"first":  {"a", "b"},
		"second": {"c"},
	}

	for _, chain := range result.Chains {
		names := make([]string, 0, len(chain.Commands()))
		for _, c := range chain.Commands() {
			names = append(names, c.Name)
		}

		expected, ok := want[chain.Name]
		if !ok {
			t.Fatalf("unexpected chain %q", chain.Name)
		}

		if strings.Join(names, ",") != strings.Join(expected, ",") {
			t.Fatalf("chain %q commands = %v, want %v", chain.Name, names, expected)
		}
	}
}
