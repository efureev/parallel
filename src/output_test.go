package parallel

import (
	"bufio"
	"bytes"
	"context"
	"testing"
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
