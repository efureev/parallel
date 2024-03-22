package parallel

import (
	"math/rand"

	"github.com/efureev/reggol"
)

func GenColors(shuffle bool) []reggol.TextStyle {
	list := []reggol.TextStyle{
		reggol.ColorFgYellow,
		reggol.ColorFgRed,
		reggol.ColorFgBlue,
		reggol.ColorFgGreen,
		reggol.ColorFgCyan,
		reggol.ColorFgMagenta,
	}

	if shuffle {
		for i := range list {
			j := rand.Intn(i + 1)
			list[i], list[j] = list[j], list[i]
		}
	}

	for _, clr := range list {
		list = append(list, clr|reggol.ColorFgBright)
	}

	return list
}
