package cli

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// parseArgs прогоняет ParseFlags с подменёнными аргументами командной строки.
func parseArgs(t *testing.T, args ...string) (*Config, error) {
	t.Helper()

	original := os.Args

	t.Cleanup(func() { os.Args = original })

	os.Args = append([]string{"parallel"}, args...)

	return ParseFlags()
}

// TestParseFlags_ChainSelection — позиционные аргументы это имена цепочек.
func TestParseFlags_ChainSelection(t *testing.T) {
	cfg, err := parseArgs(t, "api", "ui")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if strings.Join(cfg.Chains, ",") != "api,ui" {
		t.Errorf("цепочки = %v", cfg.Chains)
	}

	if len(cfg.AdHoc) != 0 {
		t.Errorf("позиционные аргументы приняты за ad-hoc: %v", cfg.AdHoc)
	}
}

// TestParseFlags_AdHocAfterDoubleDash — то же место в fs.Args(), но другой
// смысл: flag.Parse складывает позиционные аргументы и хвост после `--`
// в один срез, поэтому разделитель разбирается до него.
func TestParseFlags_AdHocAfterDoubleDash(t *testing.T) {
	cfg, err := parseArgs(t, "--", "go run ./cmd/api", "yarn dev")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if strings.Join(cfg.AdHoc, "|") != "go run ./cmd/api|yarn dev" {
		t.Errorf("ad-hoc = %v", cfg.AdHoc)
	}

	if len(cfg.Chains) != 0 {
		t.Errorf("хвост после -- принят за имена цепочек: %v", cfg.Chains)
	}
}

// TestParseFlags_FlagsBeforeDoubleDash: флаги слева от разделителя обязаны
// разбираться как флаги, а не уезжать в команды.
func TestParseFlags_FlagsBeforeDoubleDash(t *testing.T) {
	cfg, err := parseArgs(t, "-log-level", "debug", "--", "echo hi")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(cfg.AdHoc) != 1 || cfg.AdHoc[0] != "echo hi" {
		t.Errorf("ad-hoc = %v", cfg.AdHoc)
	}
}

// TestParseFlags_AdHocWithSelection: отбор относится к конфигурации, а в режиме
// ad-hoc её нет — молча проигнорировать одно из двух нельзя.
func TestParseFlags_AdHocWithSelection(t *testing.T) {
	for _, args := range [][]string{
		{"api", "--", "echo hi"},
		{"-except", "worker", "--", "echo hi"},
	} {
		if _, err := parseArgs(t, args...); !errors.Is(err, ErrAdHocWithSelection) {
			t.Errorf("%v: ожидалась ErrAdHocWithSelection, получено %v", args, err)
		}
	}
}

func TestParseFlags_Except(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "одна", raw: "worker", want: "worker"},
		{name: "несколько", raw: "worker,cron", want: "worker,cron"},
		{name: "с пробелами", raw: " worker , cron ", want: "worker,cron"},
		{name: "пусто", raw: "", want: ""},
		{name: "только запятые", raw: ",,", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseArgs(t, "-except", tt.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if got := strings.Join(cfg.Except, ","); got != tt.want {
				t.Errorf("except = %q, ожидалось %q", got, tt.want)
			}
		})
	}
}

func TestParseFlags_Modes(t *testing.T) {
	cfg, err := parseArgs(t, "-list", "-dry-run", "-no-color")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !cfg.List || !cfg.DryRun || !cfg.NoColor {
		t.Errorf("режимы не разобраны: %+v", cfg)
	}
}
