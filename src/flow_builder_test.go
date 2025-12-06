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
		"commands": CommandChainData{
			"c1": {
				"hello": {Cmd: []string{"echo", "hi"}},
			},
			"dock": {
				"ng": {Docker: &dockerCommand{Image: struct {
					Name string `yaml:"name"`
					Tag  string `yaml:"tag"`
					Pull string `yaml:"pull"`
				}{Name: "nginx"}, Ports: []string{"8080:80"}}},
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
		"commands": CommandChainData{
			"c1": {
				"enabled":  {Cmd: []string{"echo", "ok"}},
				"disabled": {Cmd: []string{"echo", "no"}, Disable: true},
			},
			"dock": {
				"ng": {Docker: &dockerCommand{Image: struct {
					Name string `yaml:"name"`
					Tag  string `yaml:"tag"`
					Pull string `yaml:"pull"`
				}{Name: "nginx"}}, Disable: true},
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
