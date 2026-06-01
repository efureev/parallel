package parallel

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestOutputFormatter_HandleOutputBasic(t *testing.T) {
	lgr := Logger()
	formatter := newOutputFormatter(lgr)

	cmd := Command{Cmd: "echo", Args: []string{"hello"}}

	var buf bytes.Buffer
	buf.WriteString("line1\nline2\n")

	reader := bufio.NewReader(&buf)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received []string
	handler := func(chainNameStyleText, cmdName, content string, counter int) {
		received = append(received, content)
	}

	if err := formatter.handleOutput(ctx, reader, cmd, handler); err != nil {
		t.Fatalf("handleOutput returned error: %v", err)
	}

	if len(received) != 2 || received[0] != "line1" || received[1] != "line2" {
		t.Fatalf("unexpected output lines: %#v", received)
	}
}

// TestOutputFormatter_HandleOutputCanceled проверяет, что чтение вывода
// прерывается отменой контекста, даже если процесс «молчит» (reader блокируется).
func TestOutputFormatter_HandleOutputCanceled(t *testing.T) {
	lgr := Logger()
	formatter := newOutputFormatter(lgr)

	cmd := Command{Cmd: "sleep", Args: []string{"100"}}

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
		done <- formatter.handleOutput(ctx, reader, cmd, handler)
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handleOutput did not return after context cancellation")
	}
}
