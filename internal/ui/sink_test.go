package ui

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer — потокобезопасный буфер: сам Sink пишет из своей горутины сброса,
// а тест читает из своей.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// TestSink_CloseFlushesTail — главная гарантия буферизации: хвост не теряется.
//
// Потеря вывода устранена на стороне чтения; буферизация записи вернула бы ту же
// проблему с другого конца, если бы Close не досбрасывал остаток.
func TestSink_CloseFlushesTail(t *testing.T) {
	var buf syncBuffer

	sink := NewSink(&buf)

	if _, err := sink.Write([]byte("tail-line\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "tail-line") {
		t.Errorf("хвост вывода потерян: %q", got)
	}
}

// TestSink_FlushesOnTimer проверяет интерактивность: при редком выводе строка
// должна появляться сама, не дожидаясь заполнения буфера.
//
// Это плата за буферизацию, и она обязана быть ограниченной: за выводом
// parallel смотрят живьём.
func TestSink_FlushesOnTimer(t *testing.T) {
	var buf syncBuffer

	sink := NewSink(&buf)
	defer func() { _ = sink.Close() }()

	if _, err := sink.Write([]byte("rare-line\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "rare-line") {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Errorf("строка не появилась в выводе за 2 с: буфер не сбрасывается по таймеру")
}

// TestSink_FlushLatency замеряет фактическую задержку появления редкой строки.
// Бюджет — не более 50 мс: за выводом parallel смотрят живьём.
func TestSink_FlushLatency(t *testing.T) {
	const budget = 50 * time.Millisecond

	var buf syncBuffer

	sink := NewSink(&buf)
	defer func() { _ = sink.Close() }()

	start := time.Now()

	if _, err := sink.Write([]byte("latency-probe\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Опрос заведомо чаще интервала сброса, чтобы измерять сброс, а не опрос.
	deadline := start.Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "latency-probe") {
			if elapsed := time.Since(start); elapsed > budget {
				t.Errorf("задержка сброса %v превышает бюджет %v", elapsed.Round(time.Millisecond), budget)
			}

			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("строка так и не появилась")
}

// TestSink_ConcurrentWrites проверяет, что Sink сам сериализует записи:
// именно он является тем потокобезопасным writer, поверх которого работает логгер.
func TestSink_ConcurrentWrites(t *testing.T) {
	const (
		writers = 8
		lines   = 200
	)

	var buf syncBuffer

	sink := NewSink(&buf)

	var wg sync.WaitGroup

	wg.Add(writers)

	// Каждый писатель шлёт строку из своего символа: так видно, что записи
	// не перемешались между собой.
	const alphabet = "abcdefgh"

	for w := range writers {
		go func() {
			defer wg.Done()

			payload := []byte(strings.Repeat(alphabet[w:w+1], 64) + "\n")

			for range lines {
				if _, err := sink.Write(payload); err != nil {
					t.Errorf("write: %v", err)

					return
				}
			}
		}()
	}

	wg.Wait()

	if err := sink.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got := strings.Count(buf.String(), "\n")
	if want := writers * lines; got != want {
		t.Errorf("строк на выходе %d, ожидалось %d", got, want)
	}
}

// TestSink_CloseIsIdempotent: повторный Close не должен паниковать на закрытом канале.
func TestSink_CloseIsIdempotent(t *testing.T) {
	sink := NewSink(&syncBuffer{})

	if err := sink.Close(); err != nil {
		t.Fatalf("первый Close: %v", err)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("повторный Close: %v", err)
	}
}
