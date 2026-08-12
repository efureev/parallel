package config

import (
	"strings"
	"testing"
)

func TestEnvPairs_SortedAndFormatted(t *testing.T) {
	got := envPairs(map[string]string{
		"ZETA":    "3",
		"ALPHA":   "1",
		"MIDDLE":  "2",
		"WITH_EQ": "a=b",
	})

	want := []string{"ALPHA=1", "MIDDLE=2", "WITH_EQ=a=b", "ZETA=3"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("envPairs = %v, want %v", got, want)
	}
}

func TestEnvPairs_Empty(t *testing.T) {
	if got := envPairs(nil); got != nil {
		t.Errorf("nil map: got %v, want nil", got)
	}

	if got := envPairs(map[string]string{}); got != nil {
		t.Errorf("empty map: got %v, want nil", got)
	}
}

// TestBuild_EnvReachesCommand проверяет сквозной путь: env из YAML доходит
// до доменной команды и для обычной команды, и для docker-режима.
func TestBuild_EnvReachesCommand(t *testing.T) {
	data := Data{
		Chains: []ChainConfig{
			{
				Name: "api",
				Commands: []NamedCommand{
					{Name: "serve", Spec: command{
						Cmd: []string{"echo", "ok"},
						Env: map[string]string{"APP_ENV": "test", "PORT": "8080"},
					}},
					{Name: "ng", Spec: command{
						Docker: &dockerCommand{Image: dockerImage("nginx")},
						Env:    map[string]string{"NGINX_HOST": "localhost"},
					}},
				},
			},
		},
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	commands := result.Chains[0].Commands()

	if got := strings.Join(commands[0].Env, ","); got != "APP_ENV=test,PORT=8080" {
		t.Errorf("regular command env = %q", got)
	}

	if got := strings.Join(commands[1].Env, ","); got != "NGINX_HOST=localhost" {
		t.Errorf("docker command env = %q", got)
	}
}

// TestUnmarshal_EnvFromYAML фиксирует имя ключа в схеме конфигурации:
// оно входит в замороженный контракт v1.
func TestUnmarshal_EnvFromYAML(t *testing.T) {
	raw := []byte("commands:\n" +
		"  api:\n" +
		"    serve:\n" +
		"      cmd: [ 'echo', 'ok' ]\n" +
		"      env:\n" +
		"        APP_ENV: production\n" +
		"        PORT: '9000'\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	spec := cfg.Chains[0].Commands[0].Spec

	if spec.Env["APP_ENV"] != "production" || spec.Env["PORT"] != "9000" {
		t.Fatalf("unexpected env: %+v", spec.Env)
	}
}
