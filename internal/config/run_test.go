package config

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

// TestBuild_RunFormWrapsInShell — строковая форма существует ради пайпов, && и
// подстановок, поэтому обязана уходить в оболочку, а не в exec напрямую.
func TestBuild_RunFormWrapsInShell(t *testing.T) {
	cmd, args, _ := buildSingle(t, command{Run: "echo hi && echo bye"})

	wantShell, wantFlag := "sh", "-c"
	if runtime.GOOS == "windows" {
		wantFlag = "/c"
	}

	if runtime.GOOS != "windows" && cmd != wantShell {
		t.Errorf("оболочка = %q, ожидалась %q", cmd, wantShell)
	}

	if len(args) != 2 || args[0] != wantFlag {
		t.Fatalf("аргументы = %v, ожидался %s + строка", args, wantFlag)
	}

	if args[1] != "echo hi && echo bye" {
		t.Errorf("строка искажена: %q", args[1])
	}
}

// TestBuild_RunFormShowsNameOnly: единственный аргумент shell-формы — вся
// команда целиком, и в префиксе каждой строки вывода он превращается в шум.
func TestBuild_RunFormShowsNameOnly(t *testing.T) {
	data := Data{Chains: []ChainConfig{{
		Name:     "c",
		Commands: []NamedCommand{{Name: "serve", Spec: command{Run: "npm run dev"}}},
	}}}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if got := result.Chains[0].Commands()[0].Format.CmdName; got != cmdNameOnly {
		t.Errorf("шаблон имени = %q, ожидался %q", got, cmdNameOnly)
	}
}

// TestBuild_RunFormKeepsExplicitFormat — умолчание не должно затирать то, что
// пользователь написал сам.
func TestBuild_RunFormKeepsExplicitFormat(t *testing.T) {
	spec := command{Run: "npm run dev"}
	spec.Format.CmdName = "%CMD_NAME% %CMD_ARGS%"

	data := Data{Chains: []ChainConfig{{
		Name:     "c",
		Commands: []NamedCommand{{Name: "serve", Spec: spec}},
	}}}

	result, err := NewFlowBuilder().Build(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if got := result.Chains[0].Commands()[0].Format.CmdName; got != "%CMD_NAME% %CMD_ARGS%" {
		t.Errorf("явный формат затёрт: %q", got)
	}
}

// TestBuild_CmdAndRunTogether: обе формы разом — недосмотр при правке, и молча
// предпочесть одну значило бы выполнить не то, что написано.
func TestBuild_CmdAndRunTogether(t *testing.T) {
	data := Data{Chains: []ChainConfig{{
		Name:     "web",
		Commands: []NamedCommand{{Name: "serve", Spec: command{Cmd: []string{"echo"}, Run: "echo hi"}}},
	}}}

	_, err := NewFlowBuilder().Build(data)
	if !errors.Is(err, ErrCmdAndRun) {
		t.Fatalf("ожидалась ErrCmdAndRun, получено %v", err)
	}

	// Без имён цепочки и команды искать место придётся глазами.
	for _, want := range []string{"web", "serve"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в сообщении нет %q: %v", want, err)
		}
	}
}

// TestUnmarshal_RunIsKnownField сторожит проведение поля через схему: без
// yaml-тега оно молча игнорировалось бы строгим разбором.
func TestUnmarshal_RunIsKnownField(t *testing.T) {
	raw := []byte("commands:\n  c:\n    t:\n      run: 'echo hi'\n")

	cfg, err := YamlFileMarshaller{}.Unmarshal(raw)
	if err != nil {
		t.Fatalf("поле run отвергнуто: %v", err)
	}

	if got := cfg.Chains[0].Commands[0].Spec.Run; got != "echo hi" {
		t.Errorf("run = %q, ожидалось %q", got, "echo hi")
	}
}
