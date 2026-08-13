package flow

import (
	"errors"
	"time"
)

// ErrEmptyCommand возвращается Command.Validate, когда не задан исполняемый файл.
var ErrEmptyCommand = errors.New("command cannot be empty")

// Format задаёт шаблон отображаемого имени команды.
type Format struct {
	CmdName string
}

// Command — одна команда внутри цепочки.
type Command struct {
	Name    string
	Cmd     string
	Args    []string
	Dir     string
	Pipe    bool
	Disable bool
	Format  Format
	// Env — переменные окружения команды в виде "KEY=VALUE".
	// Они дополняют окружение процесса, а не заменяют его: перечислять
	// весь PATH ради одной переменной никто не станет.
	Env []string
	// Timeout — предел на выполнение команды; ноль означает «без предела»
	// и оставляет решение глобальному флагу -timeout.
	Timeout time.Duration

	// Restart — правило повторного запуска; пустое значение равно RestartNever.
	Restart RestartPolicy
	// RestartAttempts — предел числа запусков. Ноль означает «без предела»:
	// главный сценарий это dev-сервер, который должен подниматься сколько
	// угодно раз, а от busy-loop защищает растущая задержка, не счётчик.
	RestartAttempts int
	// RestartDelay — задержка перед первым повтором; дальше удваивается.
	RestartDelay time.Duration

	// Ready — признак, по которому команда считается готовой к использованию.
	// Указатель: отсутствие условия — нормальное состояние, а не пустое.
	Ready *ReadyCondition
}

// DisplayName возвращает имя для показа: заданное в конфигурации либо сам исполняемый файл.
func (cmd Command) DisplayName() string {
	if cmd.Name != `` {
		return cmd.Name
	}

	return cmd.Cmd
}

// Validate проверяет, что команда пригодна к запуску.
func (cmd Command) Validate() error {
	if cmd.Cmd == "" {
		return ErrEmptyCommand
	}

	return cmd.Ready.Validate()
}
