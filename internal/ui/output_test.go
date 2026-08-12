package ui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
)

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
	if got := ChainPrefix(nil); got != "" {
		t.Fatalf("expected empty prefix for nil chain, got %q", got)
	}
}

func TestOutputFormatter_HandleOutputBasic(t *testing.T) {
	formatter := NewOutputFormatter(Logger())

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

// TestOutputFormatter_HandleOutputCanceled проверяет, что чтение вывода
// прерывается отменой контекста, даже если процесс «молчит» (reader блокируется).
func TestOutputFormatter_HandleOutputCanceled(t *testing.T) {
	formatter := NewOutputFormatter(Logger())

	cmd := flow.Command{Cmd: "sleep", Args: []string{"100"}}

	// io.Pipe без записи моделирует молчащий процесс: ReadString блокируется.
	pr, pw := io.Pipe()
	defer pw.Close()

	reader := bufio.NewReader(pr)

	ctx, cancel := context.WithCancel(t.Context())
	handler := func(chainNameStyleText, cmdName, content string, counter int) {
		t.Errorf("handler must not be called, got content=%q", content)
	}

	done := make(chan error, 1)
	go func() {
		done <- formatter.HandleOutput(ctx, reader, nil, cmd, handler)
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HandleOutput did not return after context cancellation")
	}
}

func TestOutputFormatter_FormatChainInfo(t *testing.T) {
	formatter := NewOutputFormatter(Logger())
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
