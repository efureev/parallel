//go:build windows

package config

import "os"

// shellCommand разворачивает строку в вызов оболочки.
//
// COMSPEC учитывается, потому что именно он определяет командный процессор
// в системе; при его отсутствии остаётся cmd, который есть всегда.
func shellCommand(line string) (string, []string) {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd"
	}

	return shell, []string{"/c", line}
}
