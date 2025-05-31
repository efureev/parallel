package parallel

import (
	"math/rand"

	"github.com/efureev/reggol"
)

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

func shuffleColors(colors []reggol.TextStyle) {
	for i := range colors {
		j := rand.Intn(i + 1)
		colors[i], colors[j] = colors[j], colors[i]
	}
}

func appendBrightVariants(baseColors []reggol.TextStyle) []reggol.TextStyle {
	result := make([]reggol.TextStyle, len(baseColors), len(baseColors)*2)
	copy(result, baseColors)

	for _, color := range baseColors {
		result = append(result, color|reggol.ColorFgBright)
	}

	return result
}
