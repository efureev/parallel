//go:build windows

package runner

import "strconv"

// sleepCmdNameArgs возвращает имя и аргументы кроссплатформенной команды,
// которая просто «спит» указанное число секунд. На Windows нет утилиты `sleep`,
// поэтому используем `ping`: `ping -n N+1 127.0.0.1` ждёт примерно N секунд
// (между N+1 пакетами проходит N интервалов по ~1 секунде).
func sleepCmdNameArgs(seconds int) (string, []string) {
	return "ping", []string{"-n", strconv.Itoa(seconds + 1), "127.0.0.1"}
}
