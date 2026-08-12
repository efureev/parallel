package ui

import (
	"fmt"
	"strings"

	"github.com/efureev/reggol"

	"github.com/efureev/parallel/internal/flow"
)

// FlowReader отвечает за человеко-читаемый вывод структуры Flow,
// загруженной из YAML-конфигурации.
type FlowReader struct {
	lgr *reggol.Logger
}

func NewFlowReader(lgr *reggol.Logger) *FlowReader {
	return &FlowReader{lgr: lgr}
}

// Out печатает в лог разложенную структуру Flow так,
// чтобы было понятно, какие chains и команды будут выполняться.
func (f *FlowReader) Out(fl *flow.Flow) {
	if fl == nil {
		f.lgr.Warn().Msg("Flow is <nil>, nothing to show")

		return
	}

	if len(fl.Chains) == 0 {
		f.lgr.Warn().Msg("Flow has no chains defined")

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

			if cmd.Disable {
				b.WriteString("        Disabled : true\n")
			}

			if cmd.Format.CmdName != "" {
				b.WriteString(fmt.Sprintf("        Name : %s\n", cmd.Format.CmdName))
			}
		}
	}

	f.lgr.Info().Msg(b.String())
}
