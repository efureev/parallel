package parallel

import (
	"fmt"
	"sync"
	"time"

	"github.com/efureev/reggol"
)

var (
	loggerInstance *reggol.Logger
	once           sync.Once
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
		currColor, pipe = pipe[0], pipe[1:]
		list := i.([2]string)
		return fmt.Sprintf(`%s%s`, reggol.SetColor(list[0]+`=`, currColor, trans.IsNoColor()), list[1])
	}

	return &trans
}
