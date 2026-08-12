package ui

import (
	"bufio"
	"io"
	"sync"
	"time"
)

// Параметры буферизации вывода.
const (
	// sinkBufferSize — размер буфера записи. Строка лога с раскраской и
	// префиксами укладывается в сотни байт, так что 64 КиБ — это сотни строк
	// на один write(2).
	sinkBufferSize = 64 * 1024

	// sinkFlushInterval — как часто буфер сбрасывается принудительно.
	//
	// Это плата за интерактивность: без периодического сброса редкий вывод
	// (одна строка в секунду) копился бы в буфере и появлялся пачками, что для
	// утилиты, за выводом которой смотрят живьём, неприемлемо.
	sinkFlushInterval = 30 * time.Millisecond
)

// Sink — буферизованный приёмник вывода.
//
// Решает вторую половину находки P4. Первую — чересполосицу при записи из
// нескольких горутин — закрыл переход на reggol v1 с его SyncWriter. Осталась
// цена: каждая строка лога уходила отдельным системным вызовом write(2), и на
// потоке в сотни тысяч строк в секунду это доминирующая статья расходов.
//
// Записи буферизуются и сбрасываются либо по заполнению буфера, либо по
// таймеру. Сериализацию обеспечивает мьютекс: Sink сам является тем
// «потокобезопасным writer», которого ждёт логгер.
//
// # Поведение при переполнении
//
// Политика — блокирующий backpressure: вывод не отбрасывается никогда.
// Когда буфер заполнен, bufio сбрасывает его синхронно, и пишущая горутина
// ждёт завершения write(2). Через неё давление доходит до чтения пайпа, а
// оттуда — до самого дочернего процесса, который упрётся в заполненный пайп.
//
// Это осознанный выбор, а не побочный эффект. Для утилиты, чья работа —
// показывать вывод команд, потеря строк недопустима: молча пропавшая строка
// хуже, чем замедлившийся процесс. Прежде такой политики не существовало
// вовсе — поведение получалось само собой из отсутствия буфера (находка P8).
type Sink struct {
	mu     sync.Mutex
	buf    *bufio.Writer
	out    io.Writer
	closed bool

	// dirty означает, что в буфере есть несброшенные данные. Без этого флага
	// таймер дёргал бы Flush вхолостую на каждом тике при молчащем выводе.
	dirty bool

	stop chan struct{}
	done chan struct{}
}

// NewSink оборачивает writer буфером с периодическим сбросом.
//
// Возвращённый Sink обязан быть закрыт: Close досбрасывает остаток. Без этого
// последние строки вывода остались бы в буфере и до пользователя не дошли —
// ровно тот дефект, который фаза 3 устранила на стороне чтения.
func NewSink(out io.Writer) *Sink {
	s := &Sink{
		buf:  bufio.NewWriterSize(out, sinkBufferSize),
		out:  out,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	go s.flushLoop()

	return s
}

// Write реализует io.Writer.
func (s *Sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		// После закрытия пишем напрямую: терять вывод нельзя даже если
		// кто-то опоздал с записью.
		return s.out.Write(p)
	}

	n, err := s.buf.Write(p)
	if n > 0 {
		s.dirty = true
	}

	return n, err
}

// Flush немедленно сбрасывает накопленное.
func (s *Sink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.flushLocked()
}

func (s *Sink) flushLocked() error {
	if !s.dirty {
		return nil
	}

	s.dirty = false

	return s.buf.Flush()
}

// Close останавливает фоновый сброс и досбрасывает остаток буфера.
func (s *Sink) Close() error {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()

		return nil
	}

	s.closed = true
	s.mu.Unlock()

	close(s.stop)
	<-s.done

	s.mu.Lock()
	defer s.mu.Unlock()

	s.dirty = true // досбрасываем безусловно: остаток важнее лишнего вызова

	return s.buf.Flush()
}

// flushLoop периодически сбрасывает буфер, пока Sink не закрыт.
func (s *Sink) flushLoop() {
	defer close(s.done)

	ticker := time.NewTicker(sinkFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			_ = s.flushLocked()
			s.mu.Unlock()
		}
	}
}
