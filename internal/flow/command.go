package flow

import "errors"

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

	return nil
}
