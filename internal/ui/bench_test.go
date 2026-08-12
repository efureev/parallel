package ui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/efureev/reggol"

	"github.com/efureev/parallel/internal/flow"
)

// Базовая линия для фазы 0 апгрейда (docs/UPGRADE-SPEC.md).
// Бенчмарки пишут в io.Discard: иначе замерялась бы скорость терминала,
// а не код проекта.

const (
	benchLinesPerOp = 1000
	benchLineWidth  = 80
)

// discardLogger собирает логгер с тем же трансформером, что и продакшн,
// но с выводом в никуда.
func discardLogger() *reggol.Logger {
	trans := CreateTransformer()

	out := reggol.NewConsoleWriter(func(w *reggol.ConsoleWriter) {
		w.Out = io.Discard
		w.Trans = trans
	})

	l := reggol.New(out)

	return &l
}

// benchChainAndCommand возвращает цепочку и команду так же, как их собирает FlowBuilder.
func benchChainAndCommand() (*flow.CommandChain, flow.Command) {
	chain := &flow.CommandChain{Name: "bench", Color: reggol.ColorFgCyan}
	chain.Add(flow.Command{
		Name: "worker",
		Cmd:  "echo",
		Args: []string{"hello", "world"},
		Pipe: true,
	})

	return chain, chain.Commands()[0]
}

// benchStdoutHandler повторяет stdoutHandler из runner.streamPipes дословно,
// включая вычисление div на каждой строке: бенчмарк обязан мерить то, что
// выполняется в проде, а не улучшенную версию.
func benchStdoutHandler(lgr *reggol.Logger) OutputHandler {
	return func(chainNameStyleText, cmdName, content string, counter int) {
		div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(DividerSymbol)
		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, counter, div)
		lgr.Log().Blocks(chainNameStyleText, cmdNameStyled, content).Push()
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
	lgr := discardLogger()
	chain, cmd := benchChainAndCommand()

	chainStyled := ChainPrefix(chain)
	cmdName := CommandDisplayName(cmd)
	content := strings.Repeat("x", benchLineWidth)
	handler := benchStdoutHandler(lgr)

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
	lgr := discardLogger()
	formatter := NewOutputFormatter(lgr)
	chain, cmd := benchChainAndCommand()
	handler := benchStdoutHandler(lgr)
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
