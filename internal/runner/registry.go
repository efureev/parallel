package runner

import (
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/efureev/parallel/internal/ui"
)

// trackedProcess хранит запущенную команду вместе с каналом завершения.
// Канал done закрывается владельцем процесса (executor) ровно один раз —
// после единственного вызова cmd.Wait(). Это исключает повторный/конкурентный Wait.
type trackedProcess struct {
	cmd  *exec.Cmd
	done <-chan struct{}
}

// processRegistry отвечает за учёт и остановку запущенных процессов.
type processRegistry struct {
	mu    sync.RWMutex
	procs map[string]*trackedProcess
}

func newProcessRegistry() *processRegistry {
	return &processRegistry{procs: make(map[string]*trackedProcess)}
}

// add регистрирует процесс. done — канал, который закрывается владельцем процесса
// после завершения cmd.Wait(); registry лишь дожидается его, но сам Wait не вызывает.
func (r *processRegistry) add(key string, cmd *exec.Cmd, done <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.procs[key] = &trackedProcess{cmd: cmd, done: done}
}

func (r *processRegistry) remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.procs, key)
}

// snapshot возвращает копию текущей мапы процессов для безопасной работы без удержания мьютекса.
func (r *processRegistry) snapshot() map[string]*trackedProcess {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make(map[string]*trackedProcess, len(r.procs))
	for k, v := range r.procs {
		res[k] = v
	}

	return res
}

// stopAll останавливает все зарегистрированные процессы, отправляя им заданный сигнал,
// ожидая завершения и при необходимости выполняя принудительное убийство.
func (r *processRegistry) stopAll(lgr ui.Logger, sig os.Signal, forceKill time.Duration) {
	cmds := r.snapshot()
	if len(cmds) == 0 {
		return
	}

	lgr.Info("Stopping all running commands...")

	var wg sync.WaitGroup
	wg.Add(len(cmds))

	for key, tp := range cmds {
		go func(k string, p *trackedProcess) {
			defer wg.Done()

			c := p.cmd
			if c == nil || c.Process == nil {
				return
			}

			lgr.Debug("Sending "+sig.String()+" to command group", ui.F("cmd", k))

			if err := sendSignalToGroup(c, sig); err != nil {
				lgr.Warn("Failed to send shutdown signal to process group", ui.F("err", err), ui.F("cmd", k))
			}

			// Ждём завершения через канал владельца (без собственного Wait).
			select {
			case <-time.After(forceKill):
				lgr.Warn("Force killing command group after timeout", ui.F("cmd", k))

				if err := killProcessGroup(c); err != nil {
					lgr.Warn("Failed to kill process group", ui.F("err", err), ui.F("cmd", k))
				}

				<-p.done // дождаться завершения Wait() после Kill
			case <-p.done:
			}
		}(key, tp)
	}

	wg.Wait()
}

// killAll убивает все группы процессов немедленно, без сигнала и без ожидания.
//
// Ожидания здесь нет намеренно: метод вызывается по второму Ctrl+C, когда
// пользователь уже дал понять, что ждать не готов. Дочитывание вывода и
// завершение Wait() происходят своим чередом в supervise.
func (r *processRegistry) killAll(lgr ui.Logger) {
	cmds := r.snapshot()
	if len(cmds) == 0 {
		return
	}

	lgr.Warn("Killing all running commands immediately...")

	for key, tp := range cmds {
		if tp.cmd == nil || tp.cmd.Process == nil {
			continue
		}

		if err := killProcessGroup(tp.cmd); err != nil {
			lgr.Warn("Failed to kill process group", ui.F("err", err), ui.F("cmd", key))
		}
	}
}
