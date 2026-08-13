package ui

import (
	"fmt"
	"strings"
	"time"
)

// Статусы цепочки в итоговой сводке.
const (
	StatusOK       = "ok"
	StatusFailed   = "failed"
	StatusStopped  = "stopped"
	StatusTimedOut = "timed out"
	StatusSkipped  = "skipped"
)

// SummaryRow — строка итоговой сводки.
//
// Слой представления намеренно не знает про runner.ChainResult: статус приходит
// уже решённым, а причина — уже текстом. Иначе ui пришлось бы разбирать дерево
// ошибок исполнения, а это не его дело.
type SummaryRow struct {
	Name     string
	Status   string
	Duration time.Duration
	Reason   string
}

// PrintSummary печатает итог запуска: какая цепочка чем закончилась.
//
// Нужна ровно затем, что вывод параллельных цепочек перемешан: об отказе одной
// из пяти пользователь узнаёт единственной строкой посреди сотен чужих, и чтобы
// понять, чьей именно, приходится листать вверх.
//
// При одной цепочке сводка не печатается: там перепутать нечего, а лишний блок
// в выводе — та самая мелочь, из-за которой инструмент начинают считать шумным.
func PrintSummary(lgr Logger, rows []SummaryRow) {
	if lgr == nil || len(rows) < 2 {
		return
	}

	width := 0
	for _, row := range rows {
		width = max(width, len(row.Name))
	}

	var b strings.Builder

	b.WriteString("Summary:\n")

	for _, row := range rows {
		b.WriteString(fmt.Sprintf("  %-*s  %-9s  %s",
			width, row.Name, row.Status, formatDuration(row.Duration)))

		if row.Reason != "" {
			b.WriteString("  " + row.Reason)
		}

		b.WriteString("\n")
	}

	lgr.Info(strings.TrimRight(b.String(), "\n"))
}

// formatDuration печатает длительность коротко и с постоянной точностью.
//
// time.Duration.String даёт «1.0000001s» и «2m0.000000001s» — в столбце это
// нечитаемо, а точность ниже десятых долей секунды здесь никому не нужна.
func formatDuration(d time.Duration) string {
	const minMinutes = time.Minute

	if d >= minMinutes {
		return d.Round(time.Second).String()
	}

	return d.Round(time.Millisecond).String()
}
