package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
	// ChainName — имя цепочки в верхнем регистре, без раскраски.
	ChainName string
	// CmdName — отображаемое имя команды.
	CmdName string
	// Header — раскрашенный заголовок вида «CHAIN>» для блочного вывода.
	Header string
}

// OutputFormatter форматирует и печатает информацию о командах.
//
// Палитра и готовый разделитель принадлежат форматтеру: раскраска — забота
// этого слоя, и вычислять её на каждую строку вывода незачем.
type OutputFormatter struct {
	lgr     Logger
	palette *Palette
	divider string
}

// newOutputFormatter собирает форматтер. Решение о раскраске приходит снаружи,
// от того же назначения вывода, что и у логгера — см. NewOutput.
func newOutputFormatter(lgr Logger, colored bool) *OutputFormatter {
	palette := NewPalette(true, colored)

	return &OutputFormatter{
		lgr:     lgr,
		palette: palette,
		divider: palette.wrapStyle(reggol.ColorFgMagenta|reggol.ColorFgBright, DividerSymbol),
	}
}

// Divider возвращает готовый раскрашенный разделитель.
func (o *OutputFormatter) Divider() string {
	return o.divider
}

// FormatChainInfo готовит имена цепочки и команды для печати.
// Цепочка передаётся явно: домен не хранит обратной ссылки на неё.
func (o *OutputFormatter) FormatChainInfo(chain *flow.CommandChain, cmd flow.Command) *CommandOutput {
	if chain == nil {
		return &CommandOutput{
			ChainName: "",
			CmdName:   CommandDisplayName(cmd),
			Header:    DividerSymbol,
		}
	}

	name := strings.ToUpper(chain.Name)

	return &CommandOutput{
		ChainName: name,
		CmdName:   CommandDisplayName(cmd),
		Header:    o.palette.Wrap(chain.ColorIdx, name+DividerSymbol),
	}
}

// ChainPrefix возвращает форматированный префикс с именем цепочки и разделителем.
// Если цепочка не определена, возвращается пустая строка (вывод без раскраски).
func (o *OutputFormatter) ChainPrefix(chain *flow.CommandChain) string {
	if chain == nil {
		return ""
	}

	return o.palette.Wrap(chain.ColorIdx, strings.ToUpper(chain.Name)) + ` ` + o.divider
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

// HandleOutput читает строки из reader и передаёт их в handler с форматированием
// имени цепочки и команды.
//
// Чтение идёт прямо в этом цикле, без горутины-посредника и канала между ней и
// обработчиком. Раньше посредник был нужен, чтобы прервать блокирующий
// ReadString по отмене контекста, и стоил это двух переключений горутин на
// каждую строку вывода — находка P1. После фазы 3 отмена чтения перестала быть
// штатным путём: вывод заканчивается сам по EOF, когда процесс закрывает пайп,
// а аварийная остановка выполняется закрытием пайпа снаружи (см. runner).
// Закрытый пайп разблокирует ReadString не хуже, чем это делал select.
func (o *OutputFormatter) HandleOutput(
	ctx context.Context,
	reader *bufio.Reader,
	chain *flow.CommandChain,
	cmd flow.Command,
	handler OutputHandler,
) error {
	chainNameStyleTxt := o.ChainPrefix(chain)
	cmdName := CommandDisplayName(cmd)

	counter := 0

	for {
		line, err := reader.ReadString('\n')

		if len(line) > 0 {
			handler(chainNameStyleTxt, cmdName, strings.TrimSuffix(line, NewlineChar), counter)
			counter++
		}

		if err == nil {
			continue
		}

		// EOF — штатное окончание вывода. Закрытый пайп — аварийная остановка,
		// о которой уже сообщил тот, кто её затеял. Отменённый контекст — то же
		// самое. Ни один из трёх случаев ошибкой чтения не является.
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
			return nil
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		o.lgr.Error(err, "output read failed")

		return err
	}
}
