package runner

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestExitCode — код возврата должен нести код упавшей команды.
//
// Раньше любой отказ схлопывался в `1`, и скрипт не мог отличить «команда упала
// с кодом 2» от «конфигурация не читается».
func TestExitCode(t *testing.T) {
	const fallback = 1

	single := &ExitError{Chain: "a", Command: "one", Code: 42}
	second := &ExitError{Chain: "b", Command: "two", Code: 7}

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "без ошибки", err: nil, want: 0},
		{name: "одна упавшая команда", err: single, want: 42},
		{name: "обёрнутая ошибка", err: fmt.Errorf("chain failed: %w", single), want: 42},
		{
			name: "несколько отказов — первый по порядку",
			err:  errors.Join(single, second),
			want: 42,
		},
		{
			name: "порядок задаётся вызывающим, а не значением кода",
			err:  errors.Join(second, single),
			want: 7,
		},
		{name: "ошибка не про выполнение команды", err: ErrConfigLike, want: fallback},
		{
			name: "код вне диапазона игнорируется",
			err:  &ExitError{Chain: "a", Command: "killed", Code: -1},
			want: fallback,
		},
		{
			name: "непригодный код не заслоняет пригодный",
			err:  errors.Join(&ExitError{Code: -1}, single),
			want: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err, fallback); got != tt.want {
				t.Errorf("ExitCode = %d, want %d", got, tt.want)
			}
		})
	}
}

// ErrConfigLike изображает ошибку, не связанную с выполнением команды.
var ErrConfigLike = errors.New("something else went wrong")

// TestExitError_IsCommandExecution: код появился, но проверка через errors.Is
// продолжает работать — вызывающему, которому важен лишь факт отказа, знать
// про ExitError не нужно.
func TestExitError_IsCommandExecution(t *testing.T) {
	err := error(&ExitError{Chain: "a", Command: "one", Code: 2})

	if !errors.Is(err, ErrCommandExecution) {
		t.Error("ExitError должна опознаваться как ErrCommandExecution")
	}

	if !errors.Is(fmt.Errorf("wrapped: %w", err), ErrCommandExecution) {
		t.Error("обёрнутая ExitError должна опознаваться как ErrCommandExecution")
	}
}

func TestExitError_Message(t *testing.T) {
	err := &ExitError{Chain: "build", Command: "compile", Code: 2}

	for _, want := range []string{"build", "compile", "2"} {
		if got := err.Error(); !strings.Contains(got, want) {
			t.Errorf("сообщение %q не содержит %q", got, want)
		}
	}
}
