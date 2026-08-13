package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/runner"
	"github.com/efureev/parallel/internal/ui"
)

func TestParseFlags_Timeout(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want time.Duration
	}{
		{name: "не задан", args: nil, want: 0},
		{name: "секунды", args: []string{"-timeout", "30s"}, want: 30 * time.Second},
		{name: "минуты", args: []string{"-timeout", "5m"}, want: 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseArgs(t, tt.args...)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if cfg.CommandTimeout != tt.want {
				t.Errorf("timeout = %s, ожидалось %s", cfg.CommandTimeout, tt.want)
			}
		})
	}
}

func TestParseFlags_TimeoutRejectsBareNumber(t *testing.T) {
	if _, err := parseArgs(t, "-timeout", "30"); err == nil {
		t.Fatal("число без единицы измерения принято")
	}
}

// TestSummaryRows_TimedOut — таймаут это тоже отказ, но в сводке у него своя
// причина, и она важнее самого факта отказа.
func TestSummaryRows_TimedOut(t *testing.T) {
	results := []runner.ChainResult{
		{Name: "slow", Err: &runner.TimeoutError{Chain: "slow", Command: "hangs", Limit: time.Second}},
		{Name: "bad", Err: &runner.ExitError{Chain: "bad", Command: "boom", Code: 2}},
		{Name: "good"},
	}

	rows := summaryRows(results, false)

	want := []string{ui.StatusTimedOut, ui.StatusFailed, ui.StatusOK}
	for i, status := range want {
		if rows[i].Status != status {
			t.Errorf("%s: статус = %q, ожидался %q", rows[i].Name, rows[i].Status, status)
		}
	}

	if !strings.Contains(rows[0].Reason, "exceeded") {
		t.Errorf("причина таймаута не указана: %q", rows[0].Reason)
	}
}

// TestSummaryRows_TimeoutIsRecognisedThroughWrapping: ошибки цепочек
// объединяются, и статус должен переживать обёртку.
func TestSummaryRows_TimeoutIsRecognisedThroughWrapping(t *testing.T) {
	wrapped := errors.Join(&runner.TimeoutError{Chain: "a", Command: "x", Limit: time.Second})

	rows := summaryRows([]runner.ChainResult{{Name: "a", Err: wrapped}}, false)

	if rows[0].Status != ui.StatusTimedOut {
		t.Errorf("статус = %q, ожидался %q", rows[0].Status, ui.StatusTimedOut)
	}
}
