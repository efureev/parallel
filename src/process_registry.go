package parallel

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/efureev/reggol"
)

// Timeout для graceful shutdown конкретного процесса (перед принудительным Kill).
const forceKillTimeout = 3 * time.Second

// processRegistry отвечает за учёт и остановку запущенных процессов.
type processRegistry struct {
	mu    sync.RWMutex
	procs map[string]*exec.Cmd
}

func newProcessRegistry() *processRegistry {
	return &processRegistry{procs: make(map[string]*exec.Cmd)}
}

func (r *processRegistry) add(key string, cmd *exec.Cmd) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.procs[key] = cmd
}

func (r *processRegistry) remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.procs, key)
}

// snapshot возвращает копию текущей мапы процессов для безопасной работы без удержания мьютекса.
func (r *processRegistry) snapshot() map[string]*exec.Cmd {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make(map[string]*exec.Cmd, len(r.procs))
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

	for key, cmd := range cmds {
		go func(k string, c *exec.Cmd) {
			defer wg.Done()

			if c == nil || c.Process == nil {
				return
			}

			lgr.Debug().Str("cmd", k).Msgf("Sending %s to command group", sig.String())

			if err := sendSignalToGroup(c, sig); err != nil {
				lgr.Warn().Err(err).Str("cmd", k).Msg("Failed to send shutdown signal to process group")
			}

			ctx, cancel := context.WithTimeout(context.Background(), forceKillTimeout)
			defer cancel()

			done := make(chan error, 1)

			go func() {
				done <- c.Wait()
			}()

			select {
			case <-ctx.Done():
				lgr.Warn().Str("cmd", k).Msg("Force killing command group after timeout")

				if err := killProcessGroup(c); err != nil {
					lgr.Warn().Err(err).Str("cmd", k).Msg("Failed to kill process group")
				}

				<-done // дождаться завершения после Kill
			case err := <-done:
				if err != nil {
					lgr.Warn().Err(err).Str("cmd", k).Msg("Command finished with error during shutdown")
				}
			}
		}(key, cmd)
	}

	wg.Wait()
}

// sendSignalToGroup отправляет сигнал всей группе процессов команды.
// При ошибке получения pgid сигнал отправляется только конкретному процессу.
func sendSignalToGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// fallback: шлём сигнал только самому процессу
		return cmd.Process.Signal(sig)
	}

	return syscall.Kill(-pgid, sig)
}

// killProcessGroup принудительно убивает всю группу процессов команды с помощью SIGKILL.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}

	return syscall.Kill(-pgid, syscall.SIGKILL)
}
