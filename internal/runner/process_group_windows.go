//go:build windows

package runner

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// defaultShutdownSignal возвращает сигнал завершения по умолчанию для этой платформы.
// Функция, а не переменная: os.Signal — интерфейс, константой его не объявить,
// а глобал ради одного значения не нужен.
func defaultShutdownSignal() os.Signal { return syscall.SIGTERM }

// configureProcessGroup запускает команду в новой группе процессов Windows.
//
// Флаг обязателен не только чтобы дочерний процесс не получал консольные
// события родителя напрямую: он же — условие, при котором работает
// GenerateConsoleCtrlEvent по идентификатору группы.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// sendSignalToGroup доставляет дочерней группе консольное событие CTRL_BREAK.
//
// Прежняя реализация сразу убивала процесс с комментарием, что «единственный
// надёжный способ завершения на Windows — это Kill». Верна там была только
// половина: произвольные POSIX-сигналы Windows действительно не доставляет,
// но процесс уже запущен с CREATE_NEW_PROCESS_GROUP, а это ровно то условие,
// при котором GenerateConsoleCtrlEvent доставляет событие всей группе. Его
// обрабатывают все распространённые рантаймы — Node.js, Python, .NET, Go.
// Из-за этого на Windows graceful shutdown отсутствовал как класс: дочерние
// процессы убивались без шанса закрыть соединения и снять блокировки
// .
//
// Идентификатор группы совпадает с PID процесса, созданного с флагом новой
// группы. Сам сигнал игнорируется: CTRL_BREAK — единственное событие,
// доставляемое группе надёжно, тогда как CTRL_C зависит от того, разделяет ли
// процесс консоль с родителем.
func sendSignalToGroup(cmd *exec.Cmd, _ os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	//nolint:gosec // PID неотрицателен по построению
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))
}

// killProcessGroup принудительно завершает процесс команды.
//
// Остаётся запасным путём: вызывается по таймауту, если после CTRL_BREAK
// процесс не завершился сам. Полного аналога Kill(-pgid, SIGKILL) на Windows
// нет, поэтому убивается сам процесс.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}
