package config

import (
	"slices"
	"strings"
	"testing"
)

// buildSingle собирает Flow из одной команды и возвращает её.
func buildSingle(t *testing.T, spec command) (cmd string, args []string, env []string) {
	t.Helper()

	data := Data{
		Chains: []ChainConfig{{Name: "c", Commands: []NamedCommand{{Name: "probe", Spec: spec}}}},
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	built := result.Chains[0].Commands()[0]

	return built.Cmd, built.Args, built.Env
}

// dockerSpec — минимальная docker-команда с заданными переменными.
func dockerSpec(env map[string]string) command {
	spec := command{Docker: &dockerCommand{}, Env: env}
	spec.Docker.Image.Name = "alpine"
	spec.Docker.Image.Tag = "3"

	return spec
}

// TestDockerCommand_EnvGoesToContainer — переменные обязаны доходить до контейнера.
//
// Раньше они попадали в окружение процесса клиента docker, а клиент своё
// окружение контейнеру не передаёт: написанное рядом с секцией docker не
// доходило никуда, и утилита молча делала не то, что просили.
func TestDockerCommand_EnvGoesToContainer(t *testing.T) {
	_, args, env := buildSingle(t, dockerSpec(map[string]string{"B_VAR": "2", "A_VAR": "1"}))

	if env != nil {
		t.Errorf("окружение процесса docker заполнять не нужно: %v", env)
	}

	joined := strings.Join(args, " ")
	for _, want := range []string{"-e A_VAR=1", "-e B_VAR=2"} {
		if !strings.Contains(joined, want) {
			t.Errorf("нет %q в аргументах: %s", want, joined)
		}
	}
}

// TestDockerCommand_EnvPrecedesImage: docker трактует всё после имени образа как
// команду контейнера, поэтому флаг, уехавший за образ, превратится в аргумент
// запускаемой программы.
func TestDockerCommand_EnvPrecedesImage(t *testing.T) {
	_, args, _ := buildSingle(t, dockerSpec(map[string]string{"KEY": "value"}))

	imageIdx := slices.Index(args, "alpine:3")
	if imageIdx < 0 {
		t.Fatalf("образа нет в аргументах: %v", args)
	}

	flagIdx := slices.Index(args, "-e")
	if flagIdx < 0 {
		t.Fatalf("флага -e нет в аргументах: %v", args)
	}

	if flagIdx > imageIdx {
		t.Errorf("-e стоит после имени образа, docker примет его за команду контейнера: %v", args)
	}
}

// TestDockerCommand_EnvOrderIsStable — порядок обхода мапы в Go рандомизирован,
// а аргументы видны пользователю в предпросмотре Flow.
func TestDockerCommand_EnvOrderIsStable(t *testing.T) {
	env := map[string]string{"Z": "1", "A": "2", "M": "3"}

	_, first, _ := buildSingle(t, dockerSpec(env))
	for range 20 {
		_, again, _ := buildSingle(t, dockerSpec(env))
		if !slices.Equal(first, again) {
			t.Fatalf("порядок аргументов пляшет:\n%v\n%v", first, again)
		}
	}
}

// TestDockerCommand_WithoutEnvHasNoFlag: пустая карта не должна порождать
// ни флага, ни пустого значения.
func TestDockerCommand_WithoutEnvHasNoFlag(t *testing.T) {
	for name, env := range map[string]map[string]string{"nil": nil, "пустая": {}} {
		t.Run(name, func(t *testing.T) {
			_, args, _ := buildSingle(t, dockerSpec(env))

			if slices.Contains(args, "-e") {
				t.Errorf("лишний -e без переменных: %v", args)
			}
		})
	}
}

// TestRegularCommand_EnvStaysInProcessEnv — обычная команда работает по-прежнему:
// её переменные это окружение процесса, а не аргументы.
func TestRegularCommand_EnvStaysInProcessEnv(t *testing.T) {
	spec := command{Cmd: []string{"printenv"}, Env: map[string]string{"KEY": "value"}}

	_, args, env := buildSingle(t, spec)

	if !slices.Contains(env, "KEY=value") {
		t.Errorf("переменная не попала в окружение: %v", env)
	}

	if slices.Contains(args, "-e") {
		t.Errorf("обычной команде флаг -e добавлять нельзя: %v", args)
	}
}
