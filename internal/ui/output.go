package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/efureev/reggol"

	"github.com/efureev/parallel/internal/flow"
)

// Константы оформления вывода.
const (
	cmdNameTemplate = "%CMD_NAME%"
	cmdArgsTemplate = "%CMD_ARGS%"

	// DividerSymbol разделяет имя цепочки и имя команды в выводе.
	DividerSymbol = ">"
	// NewlineChar — перевод строки, вынесен константой ради goconst.
	NewlineChar = "\n"
)

// OutputHandler получает готовую к печати строку вывода команды.
type OutputHandler func(chainNameStyleText, cmdName, content string, counter int)

// CommandOutput — заготовки имён для печати одной команды.
type CommandOutput struct {
	ChainName string
	CmdName   string
}

// OutputFormatter форматирует и печатает информацию о командах.
type OutputFormatter struct {
	lgr *reggol.Logger
}

func NewOutputFormatter(lgr *reggol.Logger) *OutputFormatter {
	return &OutputFormatter{lgr: lgr}
}

// FormatChainInfo готовит имена цепочки и команды для печати.
// Цепочка передаётся явно: домен не хранит обратной ссылки на неё.
func (o *OutputFormatter) FormatChainInfo(chain *flow.CommandChain, cmd flow.Command) *CommandOutput {
	if chain == nil {
		return &CommandOutput{
			ChainName: "",
			CmdName:   CommandDisplayName(cmd),
		}
	}

	return &CommandOutput{
		ChainName: strings.ToUpper(chain.Name),
		CmdName:   CommandDisplayName(cmd),
	}
}

// ChainPrefix возвращает форматированный префикс с именем цепочки и разделителем.
// Если цепочка не определена, возвращается пустая строка (вывод без раскраски).
func ChainPrefix(chain *flow.CommandChain) string {
	if chain == nil {
		return ""
	}

	chainName := strings.ToUpper(chain.Name)
	div := (reggol.ColorFgMagenta | reggol.ColorFgBright).Wrap(DividerSymbol)

	return chain.Color.Wrap(chainName) + ` ` + div
}

// CommandDisplayName разворачивает шаблон отображаемого имени команды.
func CommandDisplayName(cmd flow.Command) string {
	if cmd.Format.CmdName == `` {
		return fmt.Sprintf(`%s %s`, cmd.DisplayName(), strings.Join(cmd.Args, ` `))
	}

	tlpList := [2]string{cmdNameTemplate, cmdArgsTemplate}
	valueList := [2]string{cmd.DisplayName(), strings.Join(cmd.Args, ` `)}
	result := cmd.Format.CmdName

	for idx, tpl := range tlpList {
		result = strings.ReplaceAll(result, tpl, valueList[idx])
	}

	return result
}

// FullDisplayName возвращает читаемое имя с именем цепочки, например "CHAIN > echo hello".
func FullDisplayName(chainName string, cmd flow.Command) string {
	if chainName == "" {
		return CommandDisplayName(cmd)
	}

	return fmt.Sprintf("%s > %s", chainName, CommandDisplayName(cmd))
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
				case lines <- strings.TrimSuffix(str, NewlineChar):
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

// HandleOutput читает строки из reader и передаёт их в handler с форматированием
// имени цепочки и команды.
func (o *OutputFormatter) HandleOutput(
	ctx context.Context,
	reader *bufio.Reader,
	chain *flow.CommandChain,
	cmd flow.Command,
	handler OutputHandler,
) error {
	chainNameStyleTxt := ChainPrefix(chain)
	cmdName := CommandDisplayName(cmd)
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
