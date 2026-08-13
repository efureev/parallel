package config

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// dockerArgsFor собирает аргументы docker для заданной секции.
func dockerArgsFor(t *testing.T, spec *dockerCommand, lookup map[string]string, base string) []string {
	t.Helper()

	args, err := dockerArgs("box", spec, nil, lookup, dirResolver(base))
	if err != nil {
		t.Fatalf("dockerArgs: %v", err)
	}

	return args
}

// newDockerSpec — минимальная секция docker с образом.
func newDockerSpec() *dockerCommand {
	spec := &dockerCommand{}
	spec.Image.Name = "alpine"
	spec.Image.Tag = "3"

	return spec
}

// TestDockerArgs_Order — порядок здесь требование самого docker: всё после
// имени образа он считает командой контейнера.
func TestDockerArgs_Order(t *testing.T) {
	spec := newDockerSpec()
	spec.Ports = []string{"8080:80"}
	spec.Volumes = []string{"named:/mnt"}
	spec.Network = "app-net"
	spec.Args = []string{"sh", "-c", "echo hi"}

	args := dockerArgsFor(t, spec, nil, "")

	image := slices.Index(args, "alpine:3")
	if image < 0 {
		t.Fatalf("образа нет в аргументах: %v", args)
	}

	for _, flag := range []string{"-p", "-v", "--network", "--rm"} {
		if idx := slices.Index(args, flag); idx < 0 || idx > image {
			t.Errorf("флаг %s должен идти до имени образа, получено %v", flag, args)
		}
	}

	if got := strings.Join(args[image+1:], " "); got != "sh -c echo hi" {
		t.Errorf("команда контейнера = %q, ожидалась строго после образа", got)
	}
}

// TestDockerArgs_RelativeVolumeResolved — том лежит рядом с конфигурацией,
// а не там, откуда её запускают.
func TestDockerArgs_RelativeVolumeResolved(t *testing.T) {
	base := t.TempDir()

	spec := newDockerSpec()
	spec.Volumes = []string{
		"./data:/mnt/data",
		"../shared:/mnt/shared",
		"named-vol:/mnt/named",
		"/abs/path:/mnt/abs",
		"/mnt/anonymous",
	}

	joined := strings.Join(dockerArgsFor(t, spec, nil, base), " ")

	want := []string{
		filepath.Join(base, "data") + ":/mnt/data",
		filepath.Join(filepath.Dir(base), "shared") + ":/mnt/shared",
		// Именованный том — это имя, а не каталог: превратив его в путь,
		// мы сломали бы named volumes.
		"named-vol:/mnt/named",
		"/abs/path:/mnt/abs",
		"/mnt/anonymous",
	}

	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Errorf("нет тома %q в: %s", w, joined)
		}
	}
}

// TestDockerArgs_Expansion: подстановка работает и в docker-полях.
func TestDockerArgs_Expansion(t *testing.T) {
	lookup := map[string]string{"TAG": "17", "PORT": "5432", "NET": "app-net", "DATA": "/srv/data"}

	spec := &dockerCommand{}
	spec.Image.Name = "postgres"
	spec.Image.Tag = "${TAG}"
	spec.Ports = []string{"${PORT}:5432"}
	spec.Volumes = []string{"${DATA}:/var/lib/postgresql/data"}
	spec.Network = "${NET}"
	spec.Args = []string{"-c", "max_connections=${MAX:-200}"}

	joined := strings.Join(dockerArgsFor(t, spec, lookup, ""), " ")

	for _, want := range []string{
		"postgres:17", "-p 5432:5432", "-v /srv/data:/var/lib/postgresql/data",
		"--network app-net", "max_connections=200",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("нет %q в: %s", want, joined)
		}
	}
}

// TestDockerArgs_EnvNotExpandedTwice — переменные уже прошли подстановку
// в commandEnv, и повторная раскрыла бы литеральную ${...} второй раз.
func TestDockerArgs_EnvNotExpandedTwice(t *testing.T) {
	args, err := dockerArgs("box", newDockerSpec(),
		map[string]string{"TPL": "${LITERAL}"}, map[string]string{}, dirResolver(""))
	if err != nil {
		t.Fatalf("dockerArgs: %v", err)
	}

	if joined := strings.Join(args, " "); !strings.Contains(joined, "-e TPL=${LITERAL}") {
		t.Errorf("значение раскрыто повторно: %s", joined)
	}
}

// TestDockerArgs_Defaults сторожит совместимость: конфигурации без новых полей
// обязаны собирать ровно ту же команду, что и раньше.
func TestDockerArgs_Defaults(t *testing.T) {
	spec := &dockerCommand{}
	spec.Image.Name = "nginx"

	got := strings.Join(dockerArgsFor(t, spec, nil, ""), " ")
	if want := "run --name box --rm nginx:latest"; got != want {
		t.Errorf("получено %q, ожидалось %q", got, want)
	}
}

// TestDockerCommand_ShowsNameOnly: аргументы docker-команды собраны нами
// целиком и в префиксе каждой строки вывода превращаются в шум.
func TestDockerCommand_ShowsNameOnly(t *testing.T) {
	spec := dockerSpec(nil)

	data := Data{Chains: []ChainConfig{{
		Name:     "c",
		Commands: []NamedCommand{{Name: "box", Spec: spec}},
	}}}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if got := result.Chains[0].Commands()[0].Format.CmdName; got != cmdNameOnly {
		t.Errorf("шаблон имени = %q, ожидался %q", got, cmdNameOnly)
	}
}

// TestUnmarshal_DockerNewFields — поля проведены через схему.
func TestUnmarshal_DockerNewFields(t *testing.T) {
	raw := []byte(`
commands:
  c:
    db:
      docker:
        image:
          name: postgres
        volumes: [ './data:/var/lib/postgresql/data' ]
        network: app-net
        args: [ '-c', 'max_connections=200' ]
`)

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	spec := cfg.Chains[0].Commands[0].Spec.Docker
	if len(spec.Volumes) != 1 || spec.Network != "app-net" || len(spec.Args) != 2 {
		t.Errorf("поля разобраны неверно: %+v", spec)
	}
}
