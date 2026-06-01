//go:build !windows

package parallel

import "strconv"

// sleepCmdNameArgs возвращает имя и аргументы кроссплатформенной команды,
// которая просто «спит» указанное число секунд. На Unix это `sleep N`.
func sleepCmdNameArgs(seconds int) (string, []string) {
	return "sleep", []string{strconv.Itoa(seconds)}
}
