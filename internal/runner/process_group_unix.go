//go:build !windows

package runner

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// defaultShutdownSignal возвращает сигнал завершения по умолчанию для этой платформы.
// Функция, а не переменная: os.Signal — интерфейс, константой его не объявить,
// а глобал ради одного значения не нужен.
func defaultShutdownSignal() os.Signal { return unix.SIGTERM }

// configureProcessGroup настраивает запуск команды в собственной группе процессов,
// чтобы сигналы можно было доставлять всей группе (включая дочерние процессы).
//
// syscall.SysProcAttr здесь неизбежен: поле объявлено в os/exec именно этим
// типом. Всё остальное взаимодействие с ядром идёт через golang.org/x/sys/unix —
// пакет syscall заморожен и для нового кода не рекомендуется.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// sendSignalToGroup отправляет сигнал всей группе процессов команды.
//
// Сигнал шлётся именно группе, а не процессу: иначе внуки переживают родителя.
// При ошибке получения pgid сигнал отправляется только конкретному процессу.
func sendSignalToGroup(cmd *exec.Cmd, sig os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	s, ok := sig.(syscall.Signal)
	if !ok {
		s = unix.SIGTERM
	}

	pgid, err := unix.Getpgid(cmd.Process.Pid)
	if err != nil {
		// fallback: шлём сигнал только самому процессу
		return cmd.Process.Signal(s)
	}

	return unix.Kill(-pgid, s)
}

// killProcessGroup принудительно убивает всю группу процессов команды с помощью SIGKILL.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pgid, err := unix.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}

	return unix.Kill(-pgid, unix.SIGKILL)
}
