// Package ui отвечает за представление: логгер, форматирование вывода команд
// и человекочитаемый предпросмотр Flow.
package ui

import (
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/efureev/reggol"

	"github.com/efureev/parallel/internal/flow"
)

// TODO(2.4): синглтон уезжает — логгер будет конструироваться в cmd/parallel
// и передаваться вниз через порт ui.Logger. См. docs/UPGRADE-SPEC.md, задачи 2.4 и 2.4a.
var (
	//nolint:gochecknoglobals // global singleton logger is intentional for app-wide logging
	loggerInstance *reggol.Logger
	//nolint:gochecknoglobals // sync.Once to protect singleton initialization
	once sync.Once
)

// Logger возвращает общий для приложения логгер.
func Logger() *reggol.Logger {
	once.Do(func() {
		loggerInstance = createLogger()
	})

	return loggerInstance
}

func createLogger() *reggol.Logger {
	trans := CreateTransformer()
	output := reggol.NewConsoleWriter(func(w *reggol.ConsoleWriter) {
		w.Trans = trans
	})
	l := reggol.New(output)

	return &l
}

// CreateTransformer собирает консольный трансформер с раскраской имён полей.
func CreateTransformer() *reggol.ConsoleTransformer {
	trans := reggol.NewConsoleTransformer(false, time.TimeOnly)

	colorList := flow.GenColors(true)

	trans.FormatFieldFn = func(i interface{}) string {
		// Safe type handling to avoid panics on unexpected input.
		switch list := i.(type) {
		case [2]string:
			// Pick a color deterministically by hashing the key to avoid any shared mutable state.
			// This makes the transformer concurrency-safe across goroutines.
			h := fnv.New32a()
			_, _ = h.Write([]byte(list[0]))

			idx := int(h.Sum32()) % len(colorList)
			if idx < 0 {
				idx = -idx
			}

			currColor := colorList[idx]

			return fmt.Sprintf(`%s%s`, reggol.SetColor(list[0]+`=`, currColor, trans.IsNoColor()), list[1])
		default:
			return fmt.Sprintf(`%v`, i)
		}
	}

	return &trans
}
