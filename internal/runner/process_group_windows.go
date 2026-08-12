//go:build windows

package runner

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup запускает команду в новой группе процессов Windows,
// чтобы дочерний процесс не получал консольные сигналы (Ctrl+C/Ctrl+Break)
// родителя напрямую и управлялся менеджером явно.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// sendSignalToGroup на Windows не может доставлять произвольные POSIX-сигналы,
// поэтому единственный надёжный способ завершения — убить процесс.
func sendSignalToGroup(cmd *exec.Cmd, _ syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}

// killProcessGroup принудительно завершает процесс команды.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}
