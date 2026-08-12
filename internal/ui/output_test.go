package ui

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
)

// discardFormatter отдаёт форматтер без раскраски поверх отбрасывающего логгера.
func discardFormatter() *OutputFormatter {
	return NewDiscardOutput().Formatter()
}

func TestCommandDisplayName(t *testing.T) {
	cmd := flow.Command{Cmd: "echo", Args: []string{"hello"}}
	if got := CommandDisplayName(cmd); got != "echo hello" {
		t.Fatalf("unexpected CommandDisplayName result: %q", got)
	}

	cmd = flow.Command{
		Cmd:    "echo",
		Args:   []string{"hello"},
		Format: flow.Format{CmdName: "%CMD_NAME%-%CMD_ARGS%"},
	}
	if got := CommandDisplayName(cmd); got != "echo-hello" {
		t.Fatalf("unexpected formatted CommandDisplayName result: %q", got)
	}
}

func TestFullDisplayName(t *testing.T) {
	cmd := flow.Command{Cmd: "echo", Args: []string{"hi"}}

	if got := FullDisplayName("", cmd); got != "echo hi" {
		t.Fatalf("without chain: got %q", got)
	}

	if got := FullDisplayName("build", cmd); got != "build > echo hi" {
		t.Fatalf("with chain: got %q", got)
	}
}

func TestChainPrefix_NilChain(t *testing.T) {
	if got := discardFormatter().ChainPrefix(nil); got != "" {
		t.Fatalf("expected empty prefix for nil chain, got %q", got)
	}
}

func TestOutputFormatter_HandleOutputBasic(t *testing.T) {
	formatter := discardFormatter()

	cmd := flow.Command{Cmd: "echo", Args: []string{"hello"}}

	var buf bytes.Buffer
	buf.WriteString("line1\nline2\n")

	reader := bufio.NewReader(&buf)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var received []string
	handler := func(chainNameStyleText, cmdName, content string, counter int) {
		received = append(received, content)
	}

	if err := formatter.HandleOutput(ctx, reader, nil, cmd, handler); err != nil {
		t.Fatalf("HandleOutput returned error: %v", err)
	}

	if len(received) != 2 || received[0] != "line1" || received[1] != "line2" {
		t.Fatalf("unexpected output lines: %#v", received)
	}
}

// TestOutputFormatter_HandleOutputUnblocksOnClose проверяет, что чтение вывода
// прерывается, даже если процесс «молчит» и ReadString заблокирован.
//
// Гарантия та же, что и прежде, но механизм другой. Раньше блокирующее чтение
// жило в отдельной горутине и прерывалось отменой контекста через select —
// ценой двух переключений горутин на каждую строку (находка P1). Теперь чтение
// идёт прямо в цикле, а аварийная остановка выполняется закрытием пайпа:
// закрытый дескриптор разблокирует ReadString не хуже, чем select.
func TestOutputFormatter_HandleOutputUnblocksOnClose(t *testing.T) {
	formatter := discardFormatter()

	cmd := flow.Command{Cmd: "sleep", Args: []string{"100"}}

	// os.Pipe без записи моделирует молчащий процесс.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	defer func() { _ = pw.Close() }()

	reader := bufio.NewReader(pr)

	handler := func(chainNameStyleText, cmdName, content string, counter int) {
		t.Errorf("handler must not be called, got content=%q", content)
	}

	done := make(chan error, 1)

	go func() {
		done <- formatter.HandleOutput(t.Context(), reader, nil, cmd, handler)
	}()

	// Даём чтению заблокироваться, затем обрываем его закрытием пайпа.
	time.Sleep(20 * time.Millisecond)

	if err := pr.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("закрытие пайпа не должно считаться ошибкой, получено %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HandleOutput не вернулся после закрытия пайпа")
	}
}

func TestOutputFormatter_FormatChainInfo(t *testing.T) {
	formatter := discardFormatter()
	cmd := flow.Command{Name: "worker", Cmd: "echo"}

	got := formatter.FormatChainInfo(nil, cmd)
	if got.ChainName != "" {
		t.Errorf("expected empty chain name for nil chain, got %q", got.ChainName)
	}

	chain := &flow.CommandChain{Name: "build"}

	got = formatter.FormatChainInfo(chain, cmd)
	if got.ChainName != "BUILD" {
		t.Errorf("expected upper-cased chain name, got %q", got.ChainName)
	}
}
