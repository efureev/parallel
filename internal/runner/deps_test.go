package runner

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// recordingRunner запоминает момент запуска каждой команды и может держать её
// заданное время.
type recordingRunner struct {
	mu      sync.Mutex
	started map[string]time.Time

	hold    map[string]time.Duration
	failing map[string]error

	running atomic.Int32
	peak    atomic.Int32
}

func newRecordingRunner() *recordingRunner {
	return &recordingRunner{
		started: map[string]time.Time{},
		hold:    map[string]time.Duration{},
		failing: map[string]error{},
	}
}

func (r *recordingRunner) Execute(ctx context.Context, chain *flow.CommandChain, cmd flow.Command) error {
	r.mu.Lock()
	r.started[chain.Name] = time.Now()
	hold, fail := r.hold[chain.Name], r.failing[chain.Name]
	r.mu.Unlock()

	now := r.running.Add(1)
	for {
		peak := r.peak.Load()
		if now <= peak || r.peak.CompareAndSwap(peak, now) {
			break
		}
	}

	defer r.running.Add(-1)

	if hold > 0 {
		timer := time.NewTimer(hold)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fail
}

func (r *recordingRunner) ExecuteWithPipe(ctx context.Context, chain *flow.CommandChain, cmd flow.Command) error {
	return r.Execute(ctx, chain, cmd)
}

func (r *recordingRunner) startedAt(name string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.started[name]
}

// depChain собирает цепочку с зависимостями и необязательным условием готовности.
func depChain(name string, needs []string, ready *flow.ReadyCondition) *flow.CommandChain {
	chain := &flow.CommandChain{Name: name, Needs: needs}
	chain.Add(flow.Command{Name: name + "-cmd", Cmd: "echo", Ready: ready})

	return chain
}

// TestExecuteParallel_WaitsForDependency — потомок не должен стартовать раньше
// готовности предка.
func TestExecuteParallel_WaitsForDependency(t *testing.T) {
	runner := newRecordingRunner()
	runner.hold["db"] = 150 * time.Millisecond

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	chains := []*flow.CommandChain{depChain("db", nil, nil), depChain("api", []string{"db"}, nil)}

	if err := exec.ExecuteParallel(t.Context(), chains); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// У db нет условия готовности, поэтому готова она по завершении: api
	// обязан стартовать после этого момента.
	dbDone := runner.startedAt("db").Add(150 * time.Millisecond)
	if apiStart := runner.startedAt("api"); apiStart.Before(dbDone) {
		t.Errorf("api стартовал раньше готовности db: %s против %s", apiStart, dbDone)
	}
}

// TestExecuteParallel_ReadyOpensGateBeforeCompletion — главное, ради чего
// задача делалась: долгоживущий сервер не завершается никогда, и ждать его
// завершения значило бы не запустить зависимые вовсе.
func TestExecuteParallel_ReadyOpensGateBeforeCompletion(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer ln.Close()

	runner := newRecordingRunner()
	runner.hold["db"] = 2 * time.Second // «сервер» работает долго

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	ready := &flow.ReadyCondition{TCP: ln.Addr().String(), Timeout: 5 * time.Second}
	chains := []*flow.CommandChain{
		depChain("db", nil, ready),
		depChain("api", []string{"db"}, nil),
	}

	start := time.Now()

	done := make(chan error, 1)
	go func() { done <- exec.ExecuteParallel(t.Context(), chains) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("запуск не завершился")
	}

	// Порт слушается сразу, поэтому api обязан стартовать заметно раньше, чем
	// через две секунды удержания db.
	if delay := runner.startedAt("api").Sub(start); delay > time.Second {
		t.Errorf("api ждал %s — похоже, ждали завершения db, а не готовности", delay)
	}
}

// TestExecuteParallel_DependencyFailedSkips: «оборвали» и «не начинали» —
// разные вещи, и в сводке их надо различать.
func TestExecuteParallel_DependencyFailedSkips(t *testing.T) {
	runner := newRecordingRunner()
	runner.failing["db"] = errFakeA

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withKeepGoing())

	chains := []*flow.CommandChain{depChain("db", nil, nil), depChain("api", []string{"db"}, nil)}

	_ = exec.ExecuteParallel(t.Context(), chains)

	results := exec.results
	if !results[0].Failed() {
		t.Error("упавшая цепочка не помечена отказом")
	}

	if !results[1].Skipped {
		t.Errorf("зависимая цепочка должна быть skipped, получено %+v", results[1])
	}

	if !runner.startedAt("api").IsZero() {
		t.Error("зависимая цепочка запустилась, хотя предок упал")
	}

	if !errors.Is(results[1].Err, ErrDependencyFailed) {
		t.Errorf("причина не названа: %v", results[1].Err)
	}
}

// TestExecuteParallel_ReadyTimeout — сообщение обязано называть, чего именно
// не дождались.
func TestExecuteParallel_ReadyTimeout(t *testing.T) {
	runner := newRecordingRunner()
	runner.hold["db"] = 300 * time.Millisecond

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withKeepGoing())

	// Порт заведомо никем не слушается.
	ready := &flow.ReadyCondition{TCP: "127.0.0.1:1", Timeout: 100 * time.Millisecond}
	chains := []*flow.CommandChain{
		depChain("db", nil, ready),
		depChain("api", []string{"db"}, nil),
	}

	_ = exec.ExecuteParallel(t.Context(), chains)

	if !exec.results[1].Skipped {
		t.Fatalf("зависимая цепочка должна быть skipped: %+v", exec.results[1])
	}

	msg := exec.results[1].Err.Error()
	for _, want := range []string{"127.0.0.1:1", "tcp", "api"} {
		if !strings.Contains(msg, want) {
			t.Errorf("в сообщении нет %q: %s", want, msg)
		}
	}
}

// TestExecuteParallel_NoDeadlockWithLimit — ровно та взаимоблокировка, которую
// даёт errgroup.SetLimit: потомок занял бы единственный слот, ожидая предка,
// которому этот слот нужен.
func TestExecuteParallel_NoDeadlockWithLimit(t *testing.T) {
	runner := newRecordingRunner()
	runner.hold["db"] = 50 * time.Millisecond

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withMaxParallel(1))

	chains := []*flow.CommandChain{
		depChain("db", nil, nil),
		depChain("api", []string{"db"}, nil),
		depChain("worker", []string{"api"}, nil),
	}

	done := make(chan error, 1)
	go func() { done <- exec.ExecuteParallel(t.Context(), chains) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("взаимоблокировка: запуск не завершился при maxParallel=1")
	}
}

// TestExecuteParallel_RespectsLimit — одновременно работающих не больше лимита.
func TestExecuteParallel_RespectsLimit(t *testing.T) {
	runner := newRecordingRunner()

	chains := make([]*flow.CommandChain, 0, 6)
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		runner.hold[name] = 30 * time.Millisecond
		chains = append(chains, depChain(name, nil, nil))
	}

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil, withMaxParallel(2))

	if err := exec.ExecuteParallel(t.Context(), chains); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if peak := runner.peak.Load(); peak > 2 {
		t.Errorf("одновременно работало %d цепочек при лимите 2", peak)
	}
}

// TestExecuteParallel_CancelDuringWait: Ctrl+C во время ожидания готовности
// обязан выходить сразу, а не по истечении срока.
func TestExecuteParallel_CancelDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	runner := newRecordingRunner()
	runner.hold["db"] = time.Hour

	exec := newChainExecutor(ui.NewDiscardLogger(), runner, nil)

	ready := &flow.ReadyCondition{TCP: "127.0.0.1:1", Timeout: time.Hour}
	chains := []*flow.CommandChain{
		depChain("db", nil, ready),
		depChain("api", []string{"db"}, nil),
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = exec.ExecuteParallel(ctx, chains)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("отмена во время ожидания готовности не сработала")
	}
}
