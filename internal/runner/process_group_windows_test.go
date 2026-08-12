//go:build windows

package runner

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// gracefulMarker печатается дочерним процессом, если тот успел обработать
// консольное событие и завершиться сам, а не был убит.
const gracefulMarker = "GRACEFUL-EXIT"

// runGracefulChild — режим генератора для этого теста: процесс ждёт консольного
// события, печатает маркер и выходит штатно.
//
// Go на Windows доставляет и CTRL_C_EVENT, и CTRL_BREAK_EVENT как os.Interrupt.
func runGracefulChild() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	select {
	case <-ch:
		os.Stdout.WriteString(gracefulMarker + "\n")
	case <-time.After(30 * time.Second):
		os.Stdout.WriteString("TIMEOUT\n")
	}

	os.Exit(0)
}

// TestWindows_GracefulShutdownViaCtrlBreak — находка C5.
//
// До этой фазы sendSignalToGroup на Windows сразу убивал процесс, и graceful
// shutdown на трети поддерживаемых платформ отсутствовал как класс. Теперь
// дочерней группе доставляется CTRL_BREAK, и процесс получает шанс завершиться
// сам. Тест проверяет именно это: в выводе должен появиться маркер, который
// убитый процесс напечатать не успел бы.
func TestWindows_GracefulShutdownViaCtrlBreak(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	collected := make(chan string, 1)

	go func() {
		var sb strings.Builder

		buf := make([]byte, 4096)

		for {
			n, err := pr.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}

			if err != nil {
				break
			}
		}

		collected <- sb.String()
	}()

	out := ui.NewOutput(pw)
	mgr := NewManager(out.Logger(), out.Formatter(), WithTimeouts(Timeouts{
		// Запас заведомо больше времени реакции дочернего процесса: если он
		// не успеет, тест поймает убийство вместо мягкого завершения.
		ForceKill: 5 * time.Second,
		Drain:     2 * time.Second,
	}))

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	chain := &flow.CommandChain{Name: "win"}
	chain.Add(flow.Command{
		Name: "graceful",
		Cmd:  self,
		Args: []string{"-e2e.graceful-child"},
		Pipe: true,
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- mgr.ExecuteWithPipe(ctx, chain, chain.Commands()[0])
	}()

	// Даём процессу время установить обработчик, иначе событие придёт в пустоту.
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("команда не завершилась после отмены контекста")
	}

	// Досбрасываем буфер вывода до закрытия пайпа, иначе маркер не дойдёт.
	if err := out.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	_ = pw.Close()

	collectedOut := <-collected
	_ = pr.Close()

	if !strings.Contains(collectedOut, gracefulMarker) {
		t.Errorf("дочерний процесс не получил CTRL_BREAK и был убит; вывод:\n%s", collectedOut)
	}
}
