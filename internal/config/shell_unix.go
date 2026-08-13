//go:build !windows

package config

// Оболочка, через которую выполняются строковая форма run и режим ad-hoc.
const (
	shellName        = "sh"
	shellCommandFlag = "-c"
)

// shellCommand разворачивает строку в вызов оболочки.
//
// Используется полями run: и режимом ad-hoc. Оболочка нужна ровно затем, ради
// чего строку и пишут: пайпы, &&, подстановка переменных и globs.
func shellCommand(line string) (string, []string) {
	return shellName, []string{shellCommandFlag, line}
}
