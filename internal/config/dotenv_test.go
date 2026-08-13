package config

import (
	"errors"
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	raw := strings.Join([]string{
		"# комментарий целой строкой",
		"",
		"   ",
		"PROJECT=app",
		"export PORT=8080",
		`GREETING="hello world"`,
		"QUOTED='single'",
		// Знак равенства и двоеточия внутри значения — самая частая причина,
		// по которой наивный разбор ломается.
		"URL=postgres://host:5432/db?sslmode=disable&pool=10",
		"EMPTY=",
		"  SPACED  =  trimmed  ",
	}, "\n")

	env, err := parseDotEnv(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := map[string]string{
		"PROJECT":  "app",
		"PORT":     "8080",
		"GREETING": "hello world",
		"QUOTED":   "single",
		// Делим по первому знаку равенства: он часто встречается и в значениях.
		"URL":    "postgres://host:5432/db?sslmode=disable&pool=10",
		"EMPTY":  "",
		"SPACED": "trimmed",
	}

	if len(env) != len(want) {
		t.Errorf("разобрано %d переменных, ожидалось %d: %v", len(env), len(want), env)
	}

	for key, value := range want {
		if env[key] != value {
			t.Errorf("%s = %q, ожидалось %q", key, env[key], value)
		}
	}
}

// TestParseDotEnv_InlineComment — решётка без пробела встречается в паролях
// и хешах, и обрезать по ней значило бы молча портить значение.
func TestParseDotEnv_InlineComment(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "комментарий после пробела", line: "K=value # комментарий", want: "value"},
		{name: "решётка без пробела", line: "K=a#b", want: "a#b"},
		{name: "в кавычках не трогаем", line: `K="value # not a comment"`, want: "value # not a comment"},
		{name: "решётка внутри кавычек", line: `K="a#b"`, want: "a#b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := parseDotEnv(tt.line)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if env["K"] != tt.want {
				t.Errorf("K = %q, ожидалось %q", env["K"], tt.want)
			}
		})
	}
}

func TestParseDotEnv_MalformedLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantIn  string
	}{
		{name: "нет знака равенства", content: "PROJECT=app\nJUST_A_WORD\n", wantIn: "line 2"},
		{name: "пустое имя", content: "=value\n", wantIn: "empty variable name"},
		{name: "пробел в имени", content: "MY VAR=value\n", wantIn: "contains spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDotEnv(tt.content)
			if !errors.Is(err, ErrEnvFileSyntax) {
				t.Fatalf("ожидалась ErrEnvFileSyntax, получено %v", err)
			}

			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("в сообщении нет %q: %v", tt.wantIn, err)
			}
		})
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	_, err := loadDotEnv("/nonexistent/.env")
	if !errors.Is(err, ErrEnvFileRead) {
		t.Fatalf("ожидалась ErrEnvFileRead, получено %v", err)
	}

	if !strings.Contains(err.Error(), "/nonexistent/.env") {
		t.Errorf("в сообщении нет пути: %v", err)
	}
}
