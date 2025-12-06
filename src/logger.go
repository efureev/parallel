package parallel

import (
	"fmt"
	"sync"
	"time"

	"github.com/efureev/reggol"
)

var (
	//nolint:gochecknoglobals // global singleton logger is intentional for app-wide logging
	loggerInstance *reggol.Logger
	//nolint:gochecknoglobals // sync.Once to protect singleton initialization
	once sync.Once
)

func Logger() *reggol.Logger {
	once.Do(func() {
		loggerInstance = createLogger()
	})

	return loggerInstance
}

func createLogger() *reggol.Logger {
	trans := createTransformer()
	output := reggol.NewConsoleWriter(func(w *reggol.ConsoleWriter) {
		w.Trans = trans
	})
	l := reggol.New(output)

	return &l
}

func createTransformer() *reggol.ConsoleTransformer {
	trans := reggol.NewConsoleTransformer(false, time.TimeOnly)

	colorList := GenColors(true)

	var pipe []reggol.TextStyle

	trans.BeforeTransformFn = func(data reggol.EventData) {
		pipe = colorList
	}

	trans.AfterTransformFn = func(data reggol.EventData) {
		pipe = pipe[:0]
	}

	trans.FormatFieldFn = func(i interface{}) string {
		var currColor reggol.TextStyle

		if len(pipe) == 0 {
			return fmt.Sprintf(`%v`, i)
		}

		currColor, pipe = pipe[0], pipe[1:]

		// Safe type handling to avoid panics on unexpected input.
		switch list := i.(type) {
		case [2]string:
			return fmt.Sprintf(`%s%s`, reggol.SetColor(list[0]+`=`, currColor, trans.IsNoColor()), list[1])
		default:
			return fmt.Sprintf(`%v`, i)
		}
	}

	return &trans
}
