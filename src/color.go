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

// shuffleColors randomizes order of colors; crypto strength is not required here.
//
//nolint:gosec // math/rand is sufficient for UI color shuffling
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
