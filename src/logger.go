package parallel

import (
	"fmt"
	"time"

	"github.com/efureev/reggol"
)

var loggerInstance *reggol.Logger

func Logger() *reggol.Logger {
	if loggerInstance == nil {
		loggerInstance = makeLogger()
	}

	return loggerInstance
}

func makeLogger() *reggol.Logger {
	trans := reggol.NewConsoleTransformer(false, time.TimeOnly)

	// trans.HideLevel()

	// trans.FormatFieldNameFn = func(i interface{}) string {
	//	return reggol.SetColor(i, colors.FgBlue, trans.IsNoColor())
	// }
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

	output := reggol.NewConsoleWriter(func(w *reggol.ConsoleWriter) {
		w.Trans = trans
	})

	l := reggol.New(output)

	return &l
}
