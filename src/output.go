package parallel

import (
	"bufio"
	"context"
	"errors"
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

// chainPrefix возвращает форматированный префикс с именем цепочки и разделителем.
// Если цепочка не определена, возвращается пустая строка (вывод без раскраски).
func chainPrefix(chain *CommandChain) string {
	if chain == nil {
		return ""
	}

	chainName := strings.ToUpper(chain.Name)
	div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(dividerSymbol)

	return chain.Color.Wrap(chainName) + ` ` + div
}

// streamLines читает строки из reader в отдельной горутине и публикует их в каналы.
// Чтение блокирующее, поэтому вынесено из основного цикла: это позволяет немедленно
// прервать обработку при отмене ctx, не дожидаясь следующей строки от «молчащего»
// процесса (пайп закрывается при завершении/Kill).
func streamLines(ctx context.Context, reader *bufio.Reader) (<-chan string, <-chan error) {
	lines := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		for {
			str, err := reader.ReadString('\n')
			if len(str) > 0 {
				select {
				case lines <- strings.TrimSuffix(str, newlineChar):
				case <-ctx.Done():
					return
				}
			}

			if err != nil {
				errCh <- err

				return
			}
		}
	}()

	return lines, errCh
}

// handleOutput читает строки из reader и передаёт их в handler с форматированием имени цепочки и команды.
func (o *outputFormatter) handleOutput(
	ctx context.Context,
	reader *bufio.Reader,
	cmd Command,
	handler outputHandler,
) error {
	chainNameStyleTxt := chainPrefix(cmd.GetChain())
	cmdName := nameReplace(cmd)
	lines, errCh := streamLines(ctx, reader)

	counter := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line := <-lines:
			handler(chainNameStyleTxt, cmdName, line, counter)
			counter++
		case err := <-errCh:
			if errors.Is(err, io.EOF) {
				return nil
			}

			o.lgr.Err(err).Push()

			return err
		}
	}
}
