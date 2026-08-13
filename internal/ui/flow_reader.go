package ui

import (
	"fmt"
	"strings"

	"github.com/efureev/parallel/internal/flow"
)

// FlowReader отвечает за человеко-читаемый вывод структуры Flow,
// загруженной из YAML-конфигурации.
type FlowReader struct {
	lgr Logger
}

func NewFlowReader(lgr Logger) *FlowReader {
	return &FlowReader{lgr: lgr}
}

// Out печатает в лог разложенную структуру Flow так,
// чтобы было понятно, какие chains и команды будут выполняться.
func (f *FlowReader) Out(fl *flow.Flow) {
	if fl == nil {
		f.lgr.Warn("Flow is <nil>, nothing to show")

		return
	}

	if len(fl.Chains) == 0 {
		f.lgr.Warn("Flow has no chains defined")

		return
	}

	var b strings.Builder

	b.WriteString("Flow structure:" + "\n")

	for i, chain := range fl.Chains {
		b.WriteString(fmt.Sprintf("  Chain %d: %s\n", i+1, chain.Name))

		if len(chain.Needs) > 0 {
			b.WriteString(fmt.Sprintf("    Needs: %s\n", strings.Join(chain.Needs, ", ")))
		}

		commands := chain.Commands()
		if len(commands) == 0 {
			b.WriteString("    (no commands)\n")

			continue
		}

		for j, cmd := range commands {
			writeCommand(&b, j+1, cmd)
		}
	}

	writeStartOrder(&b, fl)

	f.lgr.Info(b.String())
}

// writeCommand печатает одну команду со всеми заданными полями.
func writeCommand(b *strings.Builder, num int, cmd flow.Command) {
	b.WriteString(fmt.Sprintf("    [%d] %s\n", num, cmd.DisplayName()))
	b.WriteString(fmt.Sprintf("        Exec : %s %s\n", cmd.Cmd, strings.Join(cmd.Args, " ")))

	if cmd.Dir != "" {
		b.WriteString(fmt.Sprintf("        Dir  : %s\n", cmd.Dir))
	}

	if cmd.Pipe {
		b.WriteString("        Pipe : true\n")
	}

	if cmd.Timeout > 0 {
		b.WriteString(fmt.Sprintf("        Limit: %s\n", cmd.Timeout))
	}

	if cmd.Restart != "" && cmd.Restart != flow.RestartNever {
		b.WriteString(fmt.Sprintf("        Retry: %s\n", restartSummary(cmd)))
	}

	if cmd.Ready != nil {
		b.WriteString(fmt.Sprintf("        Ready: %s, within %s\n", cmd.Ready.Describe(), cmd.Ready.Limit()))
	}

	if cmd.Disable {
		b.WriteString("        Disabled\n")
	}

	if cmd.Format.CmdName != "" {
		b.WriteString(fmt.Sprintf("        Name : %s\n", cmd.Format.CmdName))
	}
}

// writeStartOrder дописывает порядок запуска, если зависимости заданы.
//
// Без него предпросмотр отвечает только на вопрос «что запустится», а с
// зависимостями не менее важно «в каком порядке»: иначе неочевидно, почему
// цепочка стоит и чего именно ждёт.
func writeStartOrder(b *strings.Builder, fl *flow.Flow) {
	// Один уровень означает, что зависимостей нет вовсе: печатать «порядок»
	// из единственной строки — лишний шум.
	const meaningfulLevels = 2

	levels := flow.Order(*fl)
	if len(levels) < meaningfulLevels {
		return
	}

	b.WriteString("  Start order:\n")

	for i, level := range levels {
		b.WriteString(fmt.Sprintf("    %d) %s\n", i+1, strings.Join(level, ", ")))
	}
}

// restartSummary описывает политику перезапуска одной строкой.
func restartSummary(cmd flow.Command) string {
	attempts := "unlimited"
	if cmd.RestartAttempts > 0 {
		attempts = fmt.Sprintf("%d attempts", cmd.RestartAttempts)
	}

	if cmd.RestartDelay > 0 {
		return fmt.Sprintf("%s, %s, delay %s", cmd.Restart, attempts, cmd.RestartDelay)
	}

	return fmt.Sprintf("%s, %s", cmd.Restart, attempts)
}

// List печатает состав конфигурации: имена цепочек и их размер.
//
// Отдельно от Out: полный предпросмотр отвечает на вопрос «что именно
// выполнится», а этот — на вопрос «что вообще определено», с которого начинают
// в чужом проекте. Имена печатаются так, как их принимает отбор в командной
// строке, чтобы их можно было скопировать напрямую.
func (f *FlowReader) List(fl *flow.Flow) {
	if fl == nil || len(fl.Chains) == 0 {
		f.lgr.Warn("Flow has no chains defined")

		return
	}

	var b strings.Builder

	b.WriteString("Chains:\n")

	for _, chain := range fl.Chains {
		commands := chain.Commands()

		disabled := 0

		for _, cmd := range commands {
			if cmd.Disable {
				disabled++
			}
		}

		b.WriteString(fmt.Sprintf("  %s (%d)", chain.Name, len(commands)))

		if disabled > 0 {
			b.WriteString(fmt.Sprintf(", %d disabled", disabled))
		}

		if len(chain.Needs) > 0 {
			b.WriteString(fmt.Sprintf(", needs %s", strings.Join(chain.Needs, ", ")))
		}

		b.WriteString("\n")
	}

	f.lgr.Info(strings.TrimRight(b.String(), "\n"))
}
