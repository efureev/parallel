package ui

import "fmt"

// Level — уровень детальности лога.
//
// Собственный тип, а не тип библиотеки: уровень задаётся флагом командной
// строки и проходит через слой cli, который про библиотеку логирования не знает.
type Level int

// Уровни в порядке возрастания важности.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Имена уровней в том виде, в каком их принимает флаг.
const (
	levelNameDebug = "debug"
	levelNameInfo  = "info"
	levelNameWarn  = "warn"
	levelNameError = "error"
)

// String возвращает имя уровня в том виде, в каком его пишут в флаге.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return levelNameDebug
	case LevelWarn:
		return levelNameWarn
	case LevelError:
		return levelNameError
	case LevelInfo:
		return levelNameInfo
	default:
		return levelNameInfo
	}
}

// ParseLevel разбирает имя уровня.
func ParseLevel(s string) (Level, error) {
	switch s {
	case levelNameDebug:
		return LevelDebug, nil
	case levelNameInfo:
		return LevelInfo, nil
	case levelNameWarn, "warning":
		return LevelWarn, nil
	case levelNameError:
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q: expected debug, info, warn or error", s)
	}
}
