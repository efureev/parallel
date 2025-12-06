package parallel

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/efureev/reggol"
)

type outputHandler func(chainNameStyleText, cmdName, content string, counter int)

type commandOutput struct {
	chainName string
	cmdName   string
	content   string
	counter   int
}

// outputFormatter отвечает за форматирование и вывод информации о командах.
type outputFormatter struct {
	lgr *reggol.Logger
}

func newOutputFormatter(lgr *reggol.Logger) *outputFormatter {
	return &outputFormatter{lgr: lgr}
}

func (o *outputFormatter) formatChainInfo(cmd Command) *commandOutput {
	chain := cmd.GetChain()
	if chain == nil {
		return &commandOutput{
			chainName: "",
			cmdName:   nameReplace(cmd),
		}
	}

	return &commandOutput{
		chainName: strings.ToUpper(chain.Name),
		cmdName:   nameReplace(cmd),
	}
}

// handleOutput читает строки из reader и передаёт их в handler с форматированием имени цепочки и команды.
func (o *outputFormatter) handleOutput(
	ctx context.Context,
	reader *bufio.Reader,
	cmd Command,
	handler outputHandler,
) error {
	chain := cmd.GetChain()

	var (
		chainName         string
		chainNameStyle    reggol.TextStyle
		chainNameStyleTxt string
	)

	if chain != nil {
		chainName = strings.ToUpper(chain.Name)
		chainNameStyle = chain.Color
		div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(dividerSymbol)
		chainNameStyleTxt = chainNameStyle.Wrap(chainName) + ` ` + div
	} else {
		// fallback без раскраски, если цепочка не определена
		chainNameStyleTxt = ""
	}

	cmdName := nameReplace(cmd)
	counter := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		str, err := reader.ReadString('\n')
		if len(str) > 0 {
			str = strings.TrimSuffix(str, newlineChar)
			handler(chainNameStyleTxt, cmdName, str, counter)
			counter++
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			o.lgr.Err(err).Push()

			return err
		}
	}

	return nil
}
