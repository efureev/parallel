package cli

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// recorder фиксирует, какие ступени лестницы сигналов были задействованы.
type recorder struct {
	mu     sync.Mutex
	first  []os.Signal
	second int
	exited int
	done   chan struct{}
}

func newRecorder(expect int) *recorder {
	return &recorder{done: make(chan struct{}, expect)}
}

func (r *recorder) handler() signalHandler {
	return signalHandler{
		onFirst: func(sig os.Signal) {
			r.mu.Lock()
			r.first = append(r.first, sig)
			r.mu.Unlock()
			r.done <- struct{}{}
		},
		onSecond: func() {
			r.mu.Lock()
			r.second++
			r.mu.Unlock()
			r.done <- struct{}{}
		},
		exit: func(int) {
			r.mu.Lock()
			r.exited++
			r.mu.Unlock()
			r.done <- struct{}{}
		},
	}
}

// wait дожидается указанного числа реакций.
func (r *recorder) wait(t *testing.T, n int) {
	t.Helper()

	for range n {
		select {
		case <-r.done:
		case <-time.After(2 * time.Second):
			t.Fatal("обработчик сигнала не сработал вовремя")
		}
	}
}

// TestWatchSignals_Ladder — находка C3.
//
// Раньше из канала читался ровно один сигнал, после чего горутина завершалась.
// Второй и третий Ctrl+C не делали ничего, и прервать пятнадцатисекундное
// ожидание было нечем.
func TestWatchSignals_Ladder(t *testing.T) {
	sigCh := make(chan os.Signal, 3)
	rec := newRecorder(3)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go watchSignals(ctx, sigCh, rec.handler())

	sigCh <- syscall.SIGINT
	rec.wait(t, 1)

	sigCh <- syscall.SIGINT
	rec.wait(t, 1)

	sigCh <- syscall.SIGINT
	rec.wait(t, 1)

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.first) != 1 || rec.first[0] != syscall.SIGINT {
		t.Errorf("первая ступень: %v", rec.first)
	}

	if rec.second != 1 {
		t.Errorf("вторая ступень сработала %d раз, ожидался 1", rec.second)
	}

	if rec.exited != 1 {
		t.Errorf("третья ступень сработала %d раз, ожидался 1", rec.exited)
	}
}

// TestWatchSignals_StopsWithContext проверяет, что наблюдатель не переживает
// отмену контекста: иначе он жил бы до конца процесса, как прежняя горутина.
func TestWatchSignals_StopsWithContext(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	rec := newRecorder(1)

	ctx, cancel := context.WithCancel(t.Context())

	stopped := make(chan struct{})

	go func() {
		watchSignals(ctx, sigCh, rec.handler())
		close(stopped)
	}()

	cancel()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("наблюдатель за сигналами не завершился по отмене контекста")
	}
}

// TestWatchSignals_ForwardsSignalValue фиксирует, что до менеджера доходит
// именно тот сигнал, который прислал пользователь: его же и получат дочерние
// процессы.
func TestWatchSignals_ForwardsSignalValue(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	rec := newRecorder(1)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go watchSignals(ctx, sigCh, rec.handler())

	sigCh <- syscall.SIGQUIT
	rec.wait(t, 1)

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.first) != 1 || rec.first[0] != syscall.SIGQUIT {
		t.Errorf("ожидался SIGQUIT, получено %v", rec.first)
	}
}
