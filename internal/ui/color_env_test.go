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
			clearColorEnv(t)

			// NO_COLOR учитывается по факту присутствия, поэтому пустая строка
			// в наборе выше означает «переменная задана пустым значением».
			if tt.name == "NO_COLOR сильнее FORCE_COLOR" {
				t.Setenv("NO_COLOR", tt.noColorEnv)
			}

			if tt.forceEnv != "" {
				t.Setenv("FORCE_COLOR", tt.forceEnv)
			}

			if got := colorEnabled(&bytes.Buffer{}, tt.forceNoFlag); got != tt.want {
				t.Errorf("colorEnabled = %v, ожидалось %v", got, tt.want)
			}
		})
	}
}

// clearColorEnv снимает переменные раскраски на время теста и возвращает их
// прежние значения после.
//
// os.Unsetenv без восстановления протекал бы в соседние тесты пакета, а с
// -shuffle=on это давало бы отказ, зависящий от порядка запуска.
func clearColorEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{"NO_COLOR", "FORCE_COLOR"} {
		if old, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { _ = os.Setenv(key, old) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(key) })
		}

		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}

// TestNewOutput_NoColorLeavesNoEscapes — сквозная проверка: договорённость
// должна доходить до самих строк, а не оставаться в решении.
func TestNewOutput_NoColorLeavesNoEscapes(t *testing.T) {
	clearColorEnv(t)
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
