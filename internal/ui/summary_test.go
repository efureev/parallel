package ui

import (
	"strings"
	"testing"
	"time"
)

// captureLogger собирает то, что ушло в лог.
type captureLogger struct {
	Logger

	lines []string
}

func (c *captureLogger) Info(msg string, _ ...Field) { c.lines = append(c.lines, msg) }

func (c *captureLogger) text() string { return strings.Join(c.lines, "\n") }

// TestPrintSummary_SkipsSingleChain: при одной цепочке путать нечего, а лишний
// блок в выводе — та самая мелочь, из-за которой инструмент считают шумным.
func TestPrintSummary_SkipsSingleChain(t *testing.T) {
	lgr := &captureLogger{}

	PrintSummary(lgr, []SummaryRow{{Name: "api", Status: StatusOK}})

	if len(lgr.lines) != 0 {
		t.Errorf("сводка напечатана при одной цепочке: %v", lgr.lines)
	}
}

// TestPrintSummary_ShowsFailureReason — ради этого случая сводка и нужна:
// об отказе одной цепочки из пяти пользователь узнаёт единственной строкой
// посреди сотен чужих.
func TestPrintSummary_ShowsFailureReason(t *testing.T) {
	lgr := &captureLogger{}

	PrintSummary(lgr, []SummaryRow{
		{Name: "api", Status: StatusOK, Duration: 1200 * time.Millisecond},
		{Name: "worker", Status: StatusFailed, Duration: 300 * time.Millisecond, Reason: "exit status 2"},
		{Name: "ui", Status: StatusStopped, Duration: 900 * time.Millisecond},
	})

	out := lgr.text()
	for _, want := range []string{"Summary:", "api", "worker", "failed", "exit status 2", "stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("в сводке нет %q:\n%s", want, out)
		}
	}
}

// TestPrintSummary_AlignsNames: колонка нужна затем, чтобы статус читался
// взглядом сверху вниз, а не выискивался в каждой строке отдельно.
func TestPrintSummary_AlignsNames(t *testing.T) {
	lgr := &captureLogger{}

	PrintSummary(lgr, []SummaryRow{
		{Name: "a", Status: StatusOK},
		{Name: "very-long-chain", Status: StatusOK},
	})

	lines := strings.Split(lgr.text(), "\n")[1:]

	first := strings.Index(lines[0], StatusOK)
	if second := strings.Index(lines[1], StatusOK); first != second {
		t.Errorf("статусы не выровнены (%d и %d):\n%s", first, second, lgr.text())
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{in: 1234567 * time.Nanosecond, want: "1ms"},
		{in: 1500 * time.Millisecond, want: "1.5s"},
		{in: 90 * time.Second, want: "1m30s"},
	}

	for _, tt := range tests {
		if got := formatDuration(tt.in); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, ожидалось %q", tt.in, got, tt.want)
		}
	}
}
