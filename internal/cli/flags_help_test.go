package cli

import (
	"errors"
	"flag"
	"os"
	"testing"

	"github.com/efureev/parallel/internal/ui"
)

// withArgs подменяет аргументы командной строки на время теста.
func withArgs(t *testing.T, args ...string) {
	t.Helper()

	original := os.Args
	t.Cleanup(func() { os.Args = original })

	os.Args = append([]string{"parallel"}, args...)
}

// TestParseFlags_HelpIsNotAnError — просьба показать справку не сбой.
//
// Раньше `parallel -h` печатал «Failed to parse flags» и завершался с кодом 1,
// из-за чего ломал скрипты и выглядел неисправным.
func TestParseFlags_HelpIsNotAnError(t *testing.T) {
	for _, arg := range []string{"-h", "-help"} {
		t.Run(arg, func(t *testing.T) {
			withArgs(t, arg)

			devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
			if err != nil {
				t.Fatalf("open devnull: %v", err)
			}

			defer func() { _ = devNull.Close() }()

			// Справку печатать в вывод теста незачем.
			_, err = ParseFlags(func(fs *flag.FlagSet) { fs.SetOutput(devNull) })
			if !errors.Is(err, ErrHelpRequested) {
				t.Fatalf("ожидался ErrHelpRequested, получено %v", err)
			}
		})
	}
}

func TestParseFlags_LogLevel(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		want  ui.Level
		valid bool
	}{
		{name: "по умолчанию info", args: nil, want: ui.LevelInfo, valid: true},
		{name: "debug", args: []string{"-log-level", "debug"}, want: ui.LevelDebug, valid: true},
		{name: "warn", args: []string{"-log-level", "warn"}, want: ui.LevelWarn, valid: true},
		{name: "неизвестный", args: []string{"-log-level", "nope"}, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withArgs(t, tt.args...)

			cfg, err := ParseFlags()

			if !tt.valid {
				if err == nil {
					t.Fatal("неизвестный уровень принят без ошибки")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.LogLevel != tt.want {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, tt.want)
			}
		})
	}
}
