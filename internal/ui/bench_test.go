package ui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/efureev/parallel/internal/flow"
)

// Базовая линия для фазы 0 апгрейда (docs/UPGRADE-SPEC.md).
// Бенчмарки пишут в io.Discard: иначе замерялась бы скорость терминала,
// а не код проекта.

const (
	benchLinesPerOp = 1000
	benchLineWidth  = 80
)

// benchChainAndCommand возвращает цепочку и команду так же, как их собирает FlowBuilder.
func benchChainAndCommand() (*flow.CommandChain, flow.Command) {
	chain := &flow.CommandChain{Name: "bench", ColorIdx: 0}
	chain.Add(flow.Command{
		Name: "worker",
		Cmd:  "echo",
		Args: []string{"hello", "world"},
		Pipe: true,
	})

	return chain, chain.Commands()[0]
}

// benchStdoutHandler повторяет stdoutHandler из runner.streamPipes дословно:
// бенчмарк обязан мерить то, что выполняется в проде.
//
// Разделитель приходит параметром, потому что в проде он вычисляется один раз
// при создании форматтера. До фазы 2 он собирался заново на каждой строке —
// это была находка P2, снятая как побочный результат выноса раскраски в ui.
func benchStdoutHandler(lgr Logger, div string) OutputHandler {
	return func(chainNameStyleText, cmdName, content string, counter int) {
		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, counter, div)
		lgr.Blocks(chainNameStyleText, cmdNameStyled, content)
	}
}

// BenchPayload строит вывод из lines строк шириной width байт.
func BenchPayload(lines, width int) []byte {
	var buf bytes.Buffer

	buf.Grow(lines * (width + 1))

	for i := range lines {
		prefix := "line " + strconv.Itoa(i) + " "
		buf.WriteString(prefix)

		if pad := width - len(prefix); pad > 0 {
			buf.WriteString(strings.Repeat("x", pad))
		}

		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

// BenchmarkRenderLine мерит стоимость одной строки вывода: форматирование плюс отдача в логгер.
// Цель находки P2 — убрать аллокацию неизменного разделителя из этого цикла.
func BenchmarkRenderLine(b *testing.B) {
	out := NewDiscardOutput()
	lgr, formatter := out.Logger(), out.Formatter()
	chain, cmd := benchChainAndCommand()

	chainStyled := formatter.ChainPrefix(chain)
	cmdName := CommandDisplayName(cmd)
	content := strings.Repeat("x", benchLineWidth)
	handler := benchStdoutHandler(lgr, formatter.Divider())

	b.ReportAllocs()

	counter := 0
	for b.Loop() {
		handler(chainStyled, cmdName, content, counter)
		counter++
	}
}

// BenchmarkHandleOutput мерит сквозной путь вывода: чтение пайпа, канал streamLines,
// форматирование и запись. Находки P1, P4.
func BenchmarkHandleOutput(b *testing.B) {
	out := NewDiscardOutput()
	lgr, formatter := out.Logger(), out.Formatter()
	chain, cmd := benchChainAndCommand()
	handler := benchStdoutHandler(lgr, formatter.Divider())
	payload := BenchPayload(benchLinesPerOp, benchLineWidth)
	ctx := context.Background()

	b.ReportAllocs()

	iterations := 0

	for b.Loop() {
		reader := bufio.NewReader(bytes.NewReader(payload))
		if err := formatter.HandleOutput(ctx, reader, chain, cmd, handler); err != nil {
			b.Fatalf("HandleOutput: %v", err)
		}

		iterations++
	}

	perLine := float64(b.Elapsed().Nanoseconds()) / float64(iterations*benchLinesPerOp)
	b.ReportMetric(perLine, "ns/line")
	b.ReportMetric(1e9/perLine, "lines/s")
}
