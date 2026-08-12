//go:build !windows

package config

// shellCommand разворачивает строку в вызов оболочки.
//
// Используется полями run: и режимом ad-hoc. Оболочка нужна ровно затем, ради
// чего строку и пишут: пайпы, &&, подстановка переменных и globs.
func shellCommand(line string) (string, []string) {
	return "sh", []string{"-c", line}
}
