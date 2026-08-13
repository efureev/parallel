package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeFile кладёт файл и возвращает его путь.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	return path
}

// buildFrom собирает Flow из конфигурации, лежащей в указанном каталоге.
func buildFrom(t *testing.T, dir, config string) (flowCommands map[string]string, err error) {
	t.Helper()

	path := writeFile(t, dir, "flow.yaml", config)

	data, err := NewFileLoader(YamlFileMarshaller{}).Load(path)
	if err != nil {
		return nil, err
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}
	for _, cmd := range result.Chains[0].Commands() {
		out[cmd.Name] = strings.Join(cmd.Env, " ")
	}

	return out, nil
}

// TestBuild_EnvPrecedence — вся цепочка приоритета в одном тесте: окружение
// процесса → верхнеуровневые файлы → файлы команды → env.
func TestBuild_EnvPrecedence(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("LEVEL", "process")
	t.Setenv("FROM_PROCESS", "yes")

	writeFile(t, dir, ".env", "LEVEL=top-file\nONLY_TOP=top\n")
	writeFile(t, dir, ".env.cmd", "LEVEL=cmd-file\nONLY_CMD=cmd\n")

	got, err := buildFrom(t, dir, `
envFile: .env
commands:
  c:
    wins:
      cmd: [ 'echo' ]
      envFile: .env.cmd
      env:
        LEVEL: inline
    fromFiles:
      cmd: [ 'echo' ]
      envFile: .env.cmd
`)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(got["wins"], "LEVEL=inline") {
		t.Errorf("env должен быть сильнее всех: %s", got["wins"])
	}

	if !strings.Contains(got["fromFiles"], "LEVEL=cmd-file") {
		t.Errorf("файл команды должен быть сильнее верхнеуровневого: %s", got["fromFiles"])
	}

	for _, want := range []string{"ONLY_TOP=top", "ONLY_CMD=cmd"} {
		if !strings.Contains(got["fromFiles"], want) {
			t.Errorf("нет %q: %s", want, got["fromFiles"])
		}
	}

	// Переменные процесса раннер добавит сам; копировать их в каждую команду
	// незачем, иначе окружение раздувалось бы вдвое на ровном месте.
	if strings.Contains(got["fromFiles"], "FROM_PROCESS") {
		t.Errorf("окружение процесса продублировано в команду: %s", got["fromFiles"])
	}
}

// TestBuild_ExpandsEnvDirCmd — подстановка применяется ровно в трёх местах.
func TestBuild_ExpandsEnvDirCmd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "PROJECT=app\nPORT=8080\nTARGET=./cmd/api\n")

	path := writeFile(t, dir, "flow.yaml", `
envFile: .env
commands:
  c:
    t:
      dir: '${PROJECT}'
      cmd: [ 'go', 'run', '${TARGET}' ]
      env:
        URL: 'http://localhost:${PORT}'
        FALLBACK: '${NOPE:-fallback}'
`)

	data, err := NewFileLoader(YamlFileMarshaller{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	cmd := result.Chains[0].Commands()[0]

	if want := filepath.Join(dir, "app"); cmd.Dir != want {
		t.Errorf("dir = %q, ожидалось %q", cmd.Dir, want)
	}

	if !slices.Contains(cmd.Args, "./cmd/api") {
		t.Errorf("подстановка в cmd не сработала: %v", cmd.Args)
	}

	joined := strings.Join(cmd.Env, " ")
	for _, want := range []string{"URL=http://localhost:8080", "FALLBACK=fallback"} {
		if !strings.Contains(joined, want) {
			t.Errorf("нет %q: %s", want, joined)
		}
	}
}

// TestBuild_LeavesRunAlone — главное решение задачи: тело run: раскрывает
// оболочка, и второе раскрытие нашими силами дало бы либо двойное, либо
// расхождение с тем, что пользователь ждёт от $VAR.
func TestBuild_LeavesRunAlone(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "PORT=8080\n")

	path := writeFile(t, dir, "flow.yaml", `
envFile: .env
commands:
  c:
    t:
      run: 'echo ${PORT} and $PORT'
`)

	data, err := NewFileLoader(YamlFileMarshaller{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	args := result.Chains[0].Commands()[0].Args
	if got := args[len(args)-1]; got != "echo ${PORT} and $PORT" {
		t.Errorf("тело run изменено: %q", got)
	}
}

func TestBuild_MissingEnvFile(t *testing.T) {
	dir := t.TempDir()

	_, err := buildFrom(t, dir, "envFile: .env.absent\ncommands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n")
	if !errors.Is(err, ErrEnvFileRead) {
		t.Fatalf("ожидалась ErrEnvFileRead, получено %v", err)
	}

	if !strings.Contains(err.Error(), ".env.absent") {
		t.Errorf("в сообщении нет пути: %v", err)
	}
}

// TestBuild_UndefinedVariableNamesPlace: без имени команды искать место
// придётся глазами.
func TestBuild_UndefinedVariableNamesPlace(t *testing.T) {
	dir := t.TempDir()

	_, err := buildFrom(t, dir, `
commands:
  web:
    serve:
      cmd: [ 'echo' ]
      env:
        URL: '${DB_HOST}'
`)
	if !errors.Is(err, ErrUndefinedVariable) {
		t.Fatalf("ожидалась ErrUndefinedVariable, получено %v", err)
	}

	for _, want := range []string{"web", "serve", "DB_HOST"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в сообщении нет %q: %v", want, err)
		}
	}
}

// TestLoad_EnvFileRelativeToConfig — файл лежит рядом с конфигурацией, а не
// там, откуда её запускают.
func TestLoad_EnvFileRelativeToConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "FROM_FILE=yes\n")

	got, err := buildFrom(t, dir, "envFile: .env\ncommands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n")
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(got["t"], "FROM_FILE=yes") {
		t.Errorf("файл не найден относительно конфигурации: %s", got["t"])
	}
}

// TestBuild_DockerGetsMergedEnv: docker-режим превращает env в аргументы -e,
// и переменные из файла обязаны туда доехать.
func TestBuild_DockerGetsMergedEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "FROM_FILE=yes\n")

	path := writeFile(t, dir, "flow.yaml", `
envFile: .env
commands:
  c:
    ng:
      docker:
        image:
          name: nginx
`)

	data, err := NewFileLoader(YamlFileMarshaller{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if joined := strings.Join(result.Chains[0].Commands()[0].Args, " "); !strings.Contains(joined, "-e FROM_FILE=yes") {
		t.Errorf("переменная из файла не ушла контейнеру: %s", joined)
	}
}

func TestUnmarshal_EnvFileScalarAndList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "строкой", raw: "envFile: .env\n", want: []string{".env"}},
		{name: "списком", raw: "envFile: [ .env, .env.local ]\n", want: []string{".env", ".env.local"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := YamlFileMarshaller{}.Unmarshal(
				[]byte(tt.raw + "commands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n"))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if !slices.Equal(cfg.EnvFiles, tt.want) {
				t.Errorf("envFile = %v, ожидалось %v", cfg.EnvFiles, tt.want)
			}
		})
	}
}
