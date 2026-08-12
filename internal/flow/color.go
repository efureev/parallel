package flow

import (
	"math/rand"

	"github.com/efureev/reggol"
)

// TODO(2.3): палитра — забота слоя представления, а не домена. Она лежит здесь
// временно, вместе с полем CommandChain.Color, которое её потребляет: вынести
// GenColors в internal/ui отдельно от Color означало бы зависимость config → ui.
// Оба уезжают в ui одной задачей. См. docs/UPGRADE-SPEC.md, задача 2.3.

// GenColors возвращает палитру для раскраски цепочек: базовые цвета плюс их яркие варианты.
func GenColors(shuffle bool) []reggol.TextStyle {
	baseColors := getBaseColors()

	if shuffle {
		shuffleColors(baseColors)
	}

	return appendBrightVariants(baseColors)
}

func getBaseColors() []reggol.TextStyle {
	return []reggol.TextStyle{
		reggol.ColorFgYellow,
		reggol.ColorFgRed,
		reggol.ColorFgBlue,
		reggol.ColorFgGreen,
		reggol.ColorFgCyan,
		reggol.ColorFgMagenta,
	}
}

// shuffleColors перемешивает цвета; криптостойкость здесь не нужна.
//
//nolint:gosec // math/rand достаточно для выбора цвета в интерфейсе
func shuffleColors(colors []reggol.TextStyle) {
	for i := range colors {
		j := rand.Intn(i + 1)
		colors[i], colors[j] = colors[j], colors[i]
	}
}

func appendBrightVariants(baseColors []reggol.TextStyle) []reggol.TextStyle {
	const capacityFactor = 2

	result := make([]reggol.TextStyle, len(baseColors), len(baseColors)*capacityFactor)
	copy(result, baseColors)

	for _, color := range baseColors {
		result = append(result, color|reggol.ColorFgBright)
	}

	return result
}
