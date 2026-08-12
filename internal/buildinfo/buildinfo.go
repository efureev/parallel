// Package buildinfo хранит метаданные сборки, проставляемые линкером.
package buildinfo

import (
	"fmt"
	"runtime"
	"time"
)

// Эти переменные проставляются через -ldflags -X при сборке в CI.
// Значения по умолчанию рассчитаны на локальную разработку.
//
// ВАЖНО: путь к ним зашит строкой в build.sh и .github/workflows/build.yml.
// Переименование или перенос компилируются без ошибки — бинарник просто
// соберётся с пустыми метаданными. Меняешь здесь — правь оба файла.
//
//nolint:gochecknoglobals
var (
	Version = "dev"   // например v1.2.3 или вывод git describe
	Commit  = ""      // короткий SHA
	Date    = ""      // время сборки, RFC3339
	BuiltBy = "local" // CI или пользователь
)

const (
	ProjectURL       = "https://github.com/efureev/parallel"
	ProjectDescShort = "Parallel: run chains of commands concurrently with colored structured logs"
)

// Long возвращает подробный многострочный блок с информацией о версии.
func Long() string {
	bdate := Date
	if bdate == "" {
		bdate = time.Now().UTC().Format(time.RFC3339)
	}

	gover := runtime.Version()
	osarch := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	return fmt.Sprintf(`%s
Version:   %s
Commit:    %s
Built at:  %s
Built by:  %s
Go:        %s
OS/Arch:   %s
URL:       %s
`, ProjectDescShort, Version, fallback(Commit, "n/a"), bdate, fallback(BuiltBy, "n/a"), gover, osarch, ProjectURL)
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}

	return s
}
