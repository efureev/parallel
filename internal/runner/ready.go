package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/efureev/parallel/internal/flow"
)

var (
	// ErrReadyTimeout — условие готовности не выполнилось в отведённый срок.
	ErrReadyTimeout = errors.New("readiness timeout")
	// ErrDependencyFailed — предшественник не дошёл до готовности.
	ErrDependencyFailed = errors.New("dependency failed")
)

// Интервалы опроса условий готовности.
const (
	// readyPollInterval — как часто повторять пробу. Условие обычно наступает
	// в течение секунд, поэтому опрос частый; сеть от этого не страдает,
	// потому что адрес локальный.
	readyPollInterval = 100 * time.Millisecond
	// readyDialTimeout — сколько ждать одного соединения, чтобы не залипнуть
	// на неотвечающем адресе дольше самого интервала опроса.
	readyDialTimeout = time.Second
)

// gate — состояние готовности одной цепочки.
//
// Три состояния вместо булева признака: «ещё не готова», «готова» и «уже не
// будет». Последнее нужно, чтобы зависимые не ждали вечно того, что упало.
type gate struct {
	done chan struct{}
	once sync.Once
	err  error
}

func newGate() *gate {
	return &gate{done: make(chan struct{})}
}

// open переводит гейт в конечное состояние. Первый вызов побеждает: цепочка
// может одновременно и упасть, и дождаться готовности, и важнее то, что
// случилось раньше.
func (g *gate) open(err error) {
	g.once.Do(func() {
		g.err = err

		close(g.done)
	})
}

// wait ждёт готовности цепочки либо отмены.
func (g *gate) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.done:
		return g.err
	}
}

// lineMatcher следит за появлением подстроки в выводе цепочки.
type lineMatcher struct {
	substr string
	found  chan struct{}
	once   sync.Once
}

func (m *lineMatcher) observe(line string) {
	if strings.Contains(line, m.substr) {
		m.once.Do(func() { close(m.found) })
	}
}

// readySet хранит ожидания готовности всех цепочек запуска.
//
// Наблюдатели строк держатся отдельно: их дёргает слой вывода на каждой
// строке, и делать это под общим замком означало бы сериализовать весь вывод.
type readySet struct {
	gates map[string]*gate

	mu       sync.RWMutex
	matchers map[string][]*lineMatcher
}

func newReadySet(chains []*flow.CommandChain) *readySet {
	set := &readySet{
		gates:    make(map[string]*gate, len(chains)),
		matchers: make(map[string][]*lineMatcher),
	}

	for _, chain := range chains {
		set.gates[chain.Name] = newGate()
	}

	return set
}

// gateOf возвращает гейт цепочки; для неизвестного имени — уже открытый,
// чтобы отбор подмножества не приводил к вечному ожиданию.
func (s *readySet) gateOf(name string) *gate {
	if g, ok := s.gates[name]; ok {
		return g
	}

	g := newGate()
	g.open(nil)

	return g
}

// observeLine раздаёт строку вывода наблюдателям цепочки.
func (s *readySet) observeLine(chainName, line string) {
	s.mu.RLock()
	matchers := s.matchers[chainName]
	s.mu.RUnlock()

	for _, m := range matchers {
		m.observe(line)
	}
}

// addMatcher регистрирует ожидание строки и возвращает канал её появления.
func (s *readySet) addMatcher(chainName, substr string) <-chan struct{} {
	m := &lineMatcher{substr: substr, found: make(chan struct{})}

	s.mu.Lock()
	s.matchers[chainName] = append(s.matchers[chainName], m)
	s.mu.Unlock()

	return m.found
}

// awaitChain ждёт выполнения всех условий готовности цепочки.
//
// Условий может быть несколько — по одному на команду, — и цепочка считается
// готовой, когда выполнены все: поднявшийся порт при неготовой очереди готовой
// цепочкой не является.
func (s *readySet) awaitChain(ctx context.Context, chain *flow.CommandChain) error {
	var conditions []flow.Command

	for _, cmd := range chain.Commands() {
		if cmd.Ready != nil && !cmd.Disable {
			conditions = append(conditions, cmd)
		}
	}

	if len(conditions) == 0 {
		return nil
	}

	for _, cmd := range conditions {
		if err := s.awaitOne(ctx, chain.Name, cmd); err != nil {
			return err
		}
	}

	return nil
}

// awaitOne ждёт одного условия.
func (s *readySet) awaitOne(ctx context.Context, chainName string, cmd flow.Command) error {
	ready := cmd.Ready
	limit := ready.Limit()

	// Регистрировать наблюдателя надо до ожидания: строка может появиться
	// раньше, чем мы дойдём до select.
	var lines <-chan struct{}
	if ready.LogLine != "" {
		lines = s.addMatcher(chainName, ready.LogLine)
	}

	deadline, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	err := waitCondition(deadline, ready, lines)
	if err == nil {
		return nil
	}

	if errors.Is(deadline.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: chain %q waited %s for %s",
			ErrReadyTimeout, chainName, limit, ready.Describe())
	}

	return err
}

// waitCondition опрашивает условие до выполнения либо до отмены.
func waitCondition(ctx context.Context, ready *flow.ReadyCondition, lines <-chan struct{}) error {
	if lines != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-lines:
			return nil
		}
	}

	ticker := time.NewTicker(readyPollInterval)
	defer ticker.Stop()

	for {
		if probe(ctx, ready) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// probe однократно проверяет условие.
func probe(ctx context.Context, ready *flow.ReadyCondition) bool {
	switch {
	case ready.TCP != "":
		conn, err := net.DialTimeout("tcp", ready.TCP, readyDialTimeout)
		if err != nil {
			return false
		}

		_ = conn.Close()

		return true

	case len(ready.Exec) > 0:
		//nolint:gosec // команда проверки приходит из доверенной конфигурации
		cmd := exec.CommandContext(ctx, ready.Exec[0], ready.Exec[1:]...)

		return cmd.Run() == nil

	default:
		return false
	}
}
