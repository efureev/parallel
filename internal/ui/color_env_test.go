package ui

import (
	"bytes"
	"os"
	"testing"
)

// TestColorEnabled_Precedence — порядок проверок идёт от самого явного
// намерения к самому косвенному: флаг сильнее переменной окружения, переменная
// сильнее автоопределения.
//
// NO_COLOR и FORCE_COLOR — межпрограммный стандарт (no-color.org): пользователь
// задаёт их один раз на всю оболочку и вправе ждать, что утилита подчинится,
// не разбираясь в её собственных флагах.
func TestColorEnabled_Precedence(t *testing.T) {
	tests := []struct {
		name        string
		noColorEnv  string
		forceEnv    string
		forceNoFlag bool
		want        bool
	}{
		{name: "по умолчанию не терминал", want: false},
		{name: "FORCE_COLOR включает", forceEnv: "1", want: true},
		{name: "FORCE_COLOR=0 не включает", forceEnv: "0", want: false},
		{name: "NO_COLOR сильнее FORCE_COLOR", noColorEnv: "", forceEnv: "1", want: false},
		{name: "флаг сильнее всего", forceEnv: "1", forceNoFlag: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NO_COLOR учитывается по факту присутствия, поэтому пустая строка
			// в наборе выше означает «переменная задана пустым значением».
			if tt.name == "NO_COLOR сильнее FORCE_COLOR" {
				t.Setenv("NO_COLOR", tt.noColorEnv)
			} else {
				os.Unsetenv("NO_COLOR")
			}

			if tt.forceEnv != "" {
				t.Setenv("FORCE_COLOR", tt.forceEnv)
			} else {
				os.Unsetenv("FORCE_COLOR")
			}

			if got := colorEnabled(&bytes.Buffer{}, tt.forceNoFlag); got != tt.want {
				t.Errorf("colorEnabled = %v, ожидалось %v", got, tt.want)
			}
		})
	}
}

// TestNewOutput_NoColorLeavesNoEscapes — сквозная проверка: договорённость
// должна доходить до самих строк, а не оставаться в решении.
func TestNewOutput_NoColorLeavesNoEscapes(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")

	var buf bytes.Buffer

	out := NewOutput(&buf, WithoutColor())
	out.Logger().Info("hello")

	if err := out.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if bytes.Contains(buf.Bytes(), []byte("\033[")) {
		t.Errorf("в выводе остались ANSI-последовательности: %q", buf.String())
	}
}
