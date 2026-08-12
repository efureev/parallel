package runner

import (
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/efureev/reggol"
)

// Timeout для graceful shutdown конкретного процесса (перед принудительным Kill).
const forceKillTimeout = 3 * time.Second

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
func (r *processRegistry) stopAll(lgr *reggol.Logger, sig syscall.Signal) {
	cmds := r.snapshot()
	if len(cmds) == 0 {
		return
	}

	lgr.Info().Msg("Stopping all running commands...")

	var wg sync.WaitGroup
	wg.Add(len(cmds))

	for key, tp := range cmds {
		go func(k string, p *trackedProcess) {
			defer wg.Done()

			c := p.cmd
			if c == nil || c.Process == nil {
				return
			}

			lgr.Debug().Str("cmd", k).Msgf("Sending %s to command group", sig.String())

			if err := sendSignalToGroup(c, sig); err != nil {
				lgr.Warn().Err(err).Str("cmd", k).Msg("Failed to send shutdown signal to process group")
			}

			// Ждём завершения через канал владельца (без собственного Wait).
			select {
			case <-time.After(forceKillTimeout):
				lgr.Warn().Str("cmd", k).Msg("Force killing command group after timeout")

				if err := killProcessGroup(c); err != nil {
					lgr.Warn().Err(err).Str("cmd", k).Msg("Failed to kill process group")
				}

				<-p.done // дождаться завершения Wait() после Kill
			case <-p.done:
			}
		}(key, tp)
	}

	wg.Wait()
}
