package config

import (
	"errors"
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	lookup := map[string]string{"PORT": "8080", "EMPTY": ""}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "простая ссылка", in: "http://localhost:${PORT}", want: "http://localhost:8080"},
		{name: "несколько в строке", in: "${PORT}-${PORT}", want: "8080-8080"},
		{name: "умолчание не нужно", in: "${PORT:-3000}", want: "8080"},
		{name: "умолчание применяется", in: "${MISSING:-3000}", want: "3000"},
		{name: "пустое умолчание разрешено", in: "[${MISSING:-}]", want: "[]"},
		{name: "заданная пустая переменная", in: "[${EMPTY}]", want: "[]"},
		{name: "без ссылок", in: "plain text", want: "plain text"},
		// Голый доллар встречается в аргументах команд сам по себе, и съедать
		// его молча нельзя.
		{name: "голый $VAR не трогаем", in: "awk '{print $1}'", want: "awk '{print $1}'"},
		{name: "литеральная форма", in: "$${PORT}", want: "${PORT}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expand(tt.in, lookup)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}

			if got != tt.want {
				t.Errorf("expand(%q) = %q, ожидалось %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestExpand_MissingVariable — пустая строка вместо значения тихо ломает пути
// и адреса, и разбираться приходится уже по странному поведению команды.
func TestExpand_MissingVariable(t *testing.T) {
	_, err := expand("${DB_HOST}/${DB_NAME}", map[string]string{})
	if !errors.Is(err, ErrUndefinedVariable) {
		t.Fatalf("ожидалась ErrUndefinedVariable, получено %v", err)
	}

	for _, want := range []string{"DB_HOST", "DB_NAME"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("в сообщении нет %q: %v", want, err)
		}
	}
}

func TestExpandAll(t *testing.T) {
	got, err := expandAll([]string{"go", "run", "${TARGET}"}, map[string]string{"TARGET": "./cmd/api"})
	if err != nil {
		t.Fatalf("expandAll: %v", err)
	}

	if strings.Join(got, " ") != "go run ./cmd/api" {
		t.Errorf("получено %v", got)
	}
}
