//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup настраивает запуск команды в собственной группе процессов,
// чтобы сигналы можно было доставлять всей группе (включая дочерние процессы).
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
