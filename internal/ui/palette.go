package ui

import (
	"math/rand"

	"github.com/efureev/reggol"
)

// Palette раздаёт цвета цепочкам по их порядковому номеру.
//
// Палитра — забота слоя представления: домен хранит только индекс цепочки
// (flow.CommandChain.ColorIdx) и не знает ни про ANSI, ни про то, включён ли цвет.
//
// Флаг colored обязателен: без него раскраска попадёт в файл при перенаправлении
// вывода. В reggol 0.x за это отвечал глобальный флаг архивной зависимости,
// в v1 решение принимаем мы сами — см. isTerminal в logger_reggol.go.
type Palette struct {
	styles  []reggol.TextStyle
	colored bool
}

// NewPalette собирает палитру: базовые цвета плюс их яркие варианты.
// При shuffle порядок перемешивается, чтобы соседние запуски выглядели по-разному.
func NewPalette(shuffle, colored bool) *Palette {
	base := []reggol.TextStyle{
		reggol.ColorFgYellow,
		reggol.ColorFgRed,
		reggol.ColorFgBlue,
		reggol.ColorFgGreen,
		reggol.ColorFgCyan,
		reggol.ColorFgMagenta,
	}

	if shuffle {
		shuffleStyles(base)
	}

	const capacityFactor = 2

	styles := make([]reggol.TextStyle, len(base), len(base)*capacityFactor)
	copy(styles, base)

	for _, color := range base {
		styles = append(styles, color|reggol.ColorFgBright)
	}

	return &Palette{styles: styles, colored: colored}
}

// Len сообщает, сколько различных цветов знает палитра.
func (p *Palette) Len() int {
	return len(p.styles)
}

// Style возвращает цвет для цепочки с указанным номером, циклически повторяя палитру.
func (p *Palette) Style(idx int) reggol.TextStyle {
	if len(p.styles) == 0 {
		return 0
	}

	if idx < 0 {
		idx = -idx
	}

	return p.styles[idx%len(p.styles)]
}

// Wrap раскрашивает текст цветом цепочки с указанным номером.
// Если раскраска выключена, текст возвращается без изменений.
func (p *Palette) Wrap(idx int, text string) string {
	return p.wrapStyle(p.Style(idx), text)
}

// wrapStyle оборачивает текст ANSI-кодами заданного стиля.
// TextStyle.Wrap удалён в reggol 1.1.0, вместо него ColorCodes.
func (p *Palette) wrapStyle(style reggol.TextStyle, text string) string {
	if !p.colored || style == 0 {
		return text
	}

	start, reset := style.ColorCodes()

	return start + text + reset
}

// shuffleStyles перемешивает цвета; криптостойкость здесь не нужна.
//
//nolint:gosec // math/rand достаточно для выбора цвета в интерфейсе
func shuffleStyles(styles []reggol.TextStyle) {
	for i := range styles {
		j := rand.Intn(i + 1)
		styles[i], styles[j] = styles[j], styles[i]
	}
}
