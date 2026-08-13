//go:build !windows

package runner

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/ui"
)

// TestManager_TimeoutKeepsOutput — главный риск этой правки.
//
// Снятие по таймауту идёт той же лестницей, что и Ctrl+C: сигнал группе,
// затем убийство, затем ограниченное ожидание дочитывания. Обойти её прямым
// Process.Kill было бы проще, но тогда потерялся бы хвост вывода — а именно он
// и объясняет, на чём команда встала. Тест держит этот инвариант.
func TestManager_TimeoutKeepsOutput(t *testing.T) {
	requireIntegration(t)

	var sink strings.Builder

	out := ui.NewOutput(&sink)
	mgr := NewManager(out.Logger(), out.Formatter(), WithTimeouts(testTimeouts))

	chain, cmd := shCommand("chatty", "echo first; echo second; echo last-before-hang; sleep 5", true)
	cmd.Timeout = 200 * time.Millisecond

	err := mgr.ExecuteWithPipe(t.Context(), chain, cmd)
	if !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("ожидалась ErrCommandTimeout, получено %v", err)
	}

	if closeErr := out.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	printed := sink.String()
	for _, want := range []string{"first", "second", "last-before-hang"} {
		if !strings.Contains(printed, want) {
			t.Errorf("строка %q потеряна при снятии по таймауту:\n%s", want, printed)
		}
	}
}

// TestManager_TimeoutStopsChainNotSiblings: снятая по таймауту команда обрывает
// свою цепочку — это отказ, а не особый случай.
func TestManager_TimeoutStopsChain(t *testing.T) {
	requireIntegration(t)

	out := ui.NewDiscardOutput()
	mgr := NewManager(out.Logger(), out.Formatter(), WithTimeouts(testTimeouts))

	chain, cmd := shCommand("slow", "sleep 5", false)
	cmd.Timeout = 100 * time.Millisecond

	if err := mgr.Execute(t.Context(), chain, cmd); !errors.Is(err, ErrCommandTimeout) {
		t.Fatalf("ожидалась ErrCommandTimeout, получено %v", err)
	}
}
