package config

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
)

// TestUnmarshal_RestartFields — три поля доезжают до Data.
func TestUnmarshal_RestartFields(t *testing.T) {
	raw := []byte("commands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n" +
		"      restart: on-failure\n      restartAttempts: 5\n      restartDelay: 250ms\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	spec := cfg.Chains[0].Commands[0].Spec
	if spec.Restart != "on-failure" || spec.RestartAttempts != 5 || spec.RestartDelay != 250*time.Millisecond {
		t.Errorf("поля разобраны неверно: %+v", spec)
	}
}

// TestBuild_RestartReachesCommand: docker-команда такая же команда.
func TestBuild_RestartReachesCommand(t *testing.T) {
	docker := dockerSpec(nil)
	docker.Restart = "always"
	docker.RestartAttempts = 2

	data := Data{Chains: []ChainConfig{{
		Name: "c",
		Commands: []NamedCommand{
			{Name: "regular", Spec: command{Cmd: []string{"echo"}, Restart: "on-failure", RestartDelay: time.Second}},
			{Name: "container", Spec: docker},
		},
	}}}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	got := map[string]flow.Command{}
	for _, cmd := range result.Chains[0].Commands() {
		got[cmd.Name] = cmd
	}

	if got["regular"].Restart != flow.RestartOnFailure || got["regular"].RestartDelay != time.Second {
		t.Errorf("обычная команда: %+v", got["regular"])
	}

	if got["container"].Restart != flow.RestartAlways || got["container"].RestartAttempts != 2 {
		t.Errorf("docker-команда: %+v", got["container"])
	}
}

// TestBuild_RestartValidation — молча принять опечатку значило бы не
// перезапускать там, где просили.
func TestBuild_RestartValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    command
		wantErr error
		hints   []string
	}{
		{
			name:    "опечатка в политике",
			spec:    command{Cmd: []string{"echo"}, Restart: "on_failure"},
			wantErr: flow.ErrUnknownRestartPolicy,
			hints:   []string{"never", "on-failure", "always"},
		},
		{
			name:    "отрицательные попытки",
			spec:    command{Cmd: []string{"echo"}, Restart: "always", RestartAttempts: -1},
			wantErr: ErrNegativeValue,
		},
		{
			name:    "отрицательная задержка",
			spec:    command{Cmd: []string{"echo"}, Restart: "always", RestartDelay: -time.Second},
			wantErr: ErrNegativeValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := Data{Chains: []ChainConfig{{
				Name:     "web",
				Commands: []NamedCommand{{Name: "serve", Spec: tt.spec}},
			}}}

			_, err := NewFlowBuilder().Build(data)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ожидалась %v, получено %v", tt.wantErr, err)
			}

			// Контекст обязателен: без имён искать место придётся глазами.
			for _, want := range append([]string{"web", "serve"}, tt.hints...) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("в сообщении нет %q: %v", want, err)
				}
			}
		})
	}
}

// TestUnmarshal_RestartTypoHint — имена полей длинные, опечатки в них вероятны.
func TestUnmarshal_RestartTypoHint(t *testing.T) {
	raw := []byte("commands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n      restartAttemps: 3\n")

	_, err := (YamlFileMarshaller{}).Unmarshal(raw)
	if err == nil {
		t.Fatal("опечатка в имени поля принята")
	}

	if !strings.Contains(err.Error(), "restartAttempts") {
		t.Errorf("нет подсказки про restartAttempts: %v", err)
	}
}
