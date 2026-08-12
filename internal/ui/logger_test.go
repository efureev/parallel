package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNewLogger_WritesAllLevels(t *testing.T) {
	var buf bytes.Buffer

	lgr := NewLogger(&buf)

	lgr.Debug("debug line")
	lgr.Info("info line", F("key", "value"))
	lgr.Warn("warn line", F("count", 3))
	lgr.Error(errors.New("boom"), "error line")

	out := buf.String()

	for _, want := range []string{"info line", "warn line", "error line", "value", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNewLogger_Blocks(t *testing.T) {
	var buf bytes.Buffer

	lgr := NewLogger(&buf)

	lgr.Blocks("CHAIN>", "worker", "payload")
	lgr.ErrorBlocks(errors.New("failed"), "CHAIN>", "worker")

	out := buf.String()

	for _, want := range []string{"CHAIN>", "worker", "payload", "failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestNewLogger_ErrorWithoutErr проверяет, что Error можно звать без ошибки:
// так делает config.FlowBuilder, сообщая об отсутствующем поле конфигурации.
func TestNewLogger_ErrorWithoutErr(t *testing.T) {
	var buf bytes.Buffer

	NewLogger(&buf).Error(nil, "no error attached", F("field", "commands"))

	if out := buf.String(); !strings.Contains(out, "no error attached") {
		t.Errorf("output missing message:\n%s", out)
	}
}

func TestNewDiscardLogger(t *testing.T) {
	// Не должно ни паниковать, ни писать в стандартный вывод.
	lgr := NewDiscardLogger()
	lgr.Info("discarded")
	lgr.Blocks("a", "b")
}

func TestField(t *testing.T) {
	f := F("key", 42)

	if f.Key != "key" {
		t.Errorf("Key = %q, want %q", f.Key, "key")
	}

	if f.Val != 42 {
		t.Errorf("Val = %v, want 42", f.Val)
	}
}
