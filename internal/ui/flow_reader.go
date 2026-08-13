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
		chainHeader := fmt.Sprintf("  Chain %d: %s", i+1, chain.Name)
		b.WriteString(chainHeader + "\n")

		commands := chain.Commands()
		if len(commands) == 0 {
			b.WriteString("    (no commands)\n")

			continue
		}

		for j, cmd := range commands {
			b.WriteString(fmt.Sprintf("    [%d] %s\n", j+1, cmd.DisplayName()))
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

			if cmd.Disable {
				b.WriteString("        Disabled : true\n")
			}

			if cmd.Format.CmdName != "" {
				b.WriteString(fmt.Sprintf("        Name : %s\n", cmd.Format.CmdName))
			}
		}
	}

	f.lgr.Info(b.String())
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

		b.WriteString("\n")
	}

	f.lgr.Info(strings.TrimRight(b.String(), "\n"))
}
