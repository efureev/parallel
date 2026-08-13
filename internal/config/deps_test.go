package config

import (
	"strings"
	"testing"
	"time"
)

// TestUnmarshal_Needs — needs единственный ключ внутри цепочки, который не
// является именем команды.
func TestUnmarshal_Needs(t *testing.T) {
	raw := []byte("commands:\n  db:\n    x: { cmd: [ 'echo' ] }\n" +
		"  api:\n    needs: [ db ]\n    serve: { cmd: [ 'echo' ] }\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	api := cfg.Chains[1]
	if strings.Join(api.Needs, ",") != "db" {
		t.Errorf("needs = %v", api.Needs)
	}

	// Ключ не должен попасть в список команд.
	if len(api.Commands) != 1 || api.Commands[0].Name != "serve" {
		t.Errorf("needs просочился в команды: %+v", api.Commands)
	}
}

// TestUnmarshal_NeedsScalar: одиночное имя без скобок — частая форма записи.
func TestUnmarshal_NeedsScalar(t *testing.T) {
	raw := []byte("commands:\n  db:\n    x: { cmd: [ 'echo' ] }\n" +
		"  api:\n    needs: db\n    serve: { cmd: [ 'echo' ] }\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if strings.Join(cfg.Chains[1].Needs, ",") != "db" {
		t.Errorf("needs = %v", cfg.Chains[1].Needs)
	}
}

// TestUnmarshal_NeedsReservedMessage — автор конфигурации, назвавший так
// команду, должен получить объяснение, а не жалобу на тип значения.
func TestUnmarshal_NeedsReservedMessage(t *testing.T) {
	raw := []byte("commands:\n  api:\n    needs: { cmd: [ 'echo' ] }\n")

	_, err := (YamlFileMarshaller{}).Unmarshal(raw)
	if err == nil {
		t.Fatal("команда с именем needs принята без объяснения")
	}

	for _, want := range []string{"reserved", "api"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в сообщении нет %q: %v", want, err)
		}
	}
}

func TestUnmarshal_MaxParallel(t *testing.T) {
	raw := []byte("maxParallel: 3\ncommands:\n  c:\n    x: { cmd: [ 'echo' ] }\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.MaxParallel != 3 {
		t.Errorf("maxParallel = %d", cfg.MaxParallel)
	}

	negative := []byte("maxParallel: -1\ncommands:\n  c:\n    x: { cmd: [ 'echo' ] }\n")
	if _, err := (YamlFileMarshaller{}).Unmarshal(negative); err == nil {
		t.Error("отрицательный maxParallel принят")
	}
}

// TestBuild_ReadyReachesCommand: секция ready проведена через сборку, включая
// подстановку переменных.
func TestBuild_ReadyReachesCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "PORT=5432\n")

	path := writeFile(t, dir, "flow.yaml", `
envFile: .env
commands:
  db:
    postgres:
      cmd: [ 'echo' ]
      ready:
        tcp: '127.0.0.1:${PORT}'
        timeout: 5s
`)

	data, err := NewFileLoader(YamlFileMarshaller{}).Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	ready := result.Chains[0].Commands()[0].Ready
	if ready == nil {
		t.Fatal("условие готовности потеряно")
	}

	if ready.TCP != "127.0.0.1:5432" {
		t.Errorf("подстановка не сработала: %q", ready.TCP)
	}

	if ready.Timeout != 5*time.Second {
		t.Errorf("timeout = %s", ready.Timeout)
	}
}

// TestBuild_ReadyValidated — правило «ровно одно условие» принадлежит домену,
// но должно срабатывать на пути сборки.
func TestBuild_ReadyValidated(t *testing.T) {
	data := Data{Chains: []ChainConfig{{
		Name: "c",
		Commands: []NamedCommand{{Name: "x", Spec: command{
			Cmd:   []string{"echo"},
			Ready: &readyCondition{TCP: "127.0.0.1:1", LogLine: "up"},
		}}},
	}}}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if err := result.Validate(); err == nil {
		t.Error("два условия готовности приняты")
	} else if !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("невнятное сообщение: %v", err)
	}
}
