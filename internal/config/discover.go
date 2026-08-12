package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultConfigName — имя, которое ищется первым и указано как значение по
// умолчанию в справке. Входит в замороженный контракт v1.
const DefaultConfigName = ".parallelrc.yaml"

// configNames — имена файла конфигурации в порядке предпочтения.
//
// Расширение .yml добавлено не ради полноты: обе формы одинаково
// распространены, а пользователь, назвавший файл .parallelrc.yml, получал
// «config file not found» с именем, которого он не писал, — и не понимал,
// в чём отличие.
//
//nolint:gochecknoglobals // неизменяемый список, константой объявить нельзя
var configNames = []string{DefaultConfigName, ".parallelrc.yml"}

// Discover ищет файл конфигурации в startDir и выше по дереву каталогов.
//
// Подъём наверх — не удобство, а условие работоспособности: конфигурация лежит
// в корне проекта и коммитится вместе с ним, а команды запускают из любого
// подкаталога. Так же ведут себя git, docker compose и любой линтер.
//
// Поиск идёт до корня файловой системы. Ограничивать его границей репозитория
// заманчиво, но это привязало бы поведение к git — а проект может лежать и вне
// репозитория; кроме того, «почему не нашёл» тогда зависело бы от наличия
// каталога .git, что невозможно объяснить в сообщении об ошибке.
//
// Функция применяется только когда путь не задан флагом -f: явный путь ищется
// ровно там, где указан, иначе опечатка в -f молча приводила бы к запуску
// чужой конфигурации.
func Discover(startDir string) (string, error) {
	dir := startDir
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}

	for {
		for _, name := range configNames {
			candidate := filepath.Join(dir, name)
			if isFile(candidate) {
				return candidate, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return "", fmt.Errorf("%w: searched for %s in %s and every parent directory",
		ErrConfigNotFound, strings.Join(quoted(configNames), " or "), startDir)
}

// isFile отсекает каталоги: одноимённый каталог остановил бы поиск, и вместо
// подъёма выше пользователь получил бы ошибку чтения.
func isFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// quoted заключает имена в кавычки для сообщения об ошибке.
func quoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = `"` + n + `"`
	}

	return out
}
