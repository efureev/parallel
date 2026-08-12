package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/efureev/parallel/internal/flow"
)

// AdHoc собирает Flow из команд, переданных в командной строке, минуя файл.
//
// Каждая команда становится отдельной цепочкой, потому что цепочки — это и есть
// единица параллелизма: положив их в одну, мы получили бы последовательный
// запуск, а просят прямо противоположного.
//
// Все команды потоковые: запуск из командной строки — это всегда наблюдение за
// живым выводом, а не сбор блока по завершении.
func AdHoc(lines []string) (flow.Flow, error) {
	if len(lines) == 0 {
		return flow.Flow{}, ErrNoAdHocCommands
	}

	result := flow.Flow{}
	used := make(map[string]int, len(lines))

	for idx, line := range lines {
		if strings.TrimSpace(line) == "" {
			return flow.Flow{}, fmt.Errorf("%w: command %d is empty", ErrNoAdHocCommands, idx+1)
		}

		name := uniqueName(adHocName(line), used)
		cmdStr, args := shellCommand(line)

		chain := &flow.CommandChain{Name: name, ColorIdx: idx}
		chain.Add(flow.Command{
			Name: name, Cmd: cmdStr, Args: args, Pipe: true,
			// Без этого префиксом каждой строки станет вся команда целиком.
			Format: flow.Format{CmdName: cmdNameOnly},
		})

		result.AddChain(chain)
	}

	return result, nil
}

// adHocName выбирает короткое имя цепочки по тексту команды.
//
// Имя попадает в префикс каждой строки вывода, поэтому целиком команду брать
// нельзя: префикс шире самого вывода делает лог нечитаемым. Первого слова
// достаточно, чтобы отличить `go run` от `yarn dev`.
func adHocName(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "cmd"
	}

	// Ведущие присваивания вида FOO=bar пропускаем: различает команды не они.
	for _, field := range fields {
		if !strings.Contains(field, "=") {
			return lastPathElement(field)
		}
	}

	return lastPathElement(fields[0])
}

// lastPathElement оставляет от ./cmd/api или /usr/bin/php только последний элемент.
func lastPathElement(s string) string {
	if idx := strings.LastIndexAny(s, `/\`); idx >= 0 && idx+1 < len(s) {
		return s[idx+1:]
	}

	return s
}

// uniqueName разводит одинаковые имена номерами: две команды `go run ./a` и
// `go run ./b` иначе дали бы неразличимые префиксы в выводе.
func uniqueName(base string, used map[string]int) string {
	used[base]++

	if n := used[base]; n > 1 {
		return base + "-" + strconv.Itoa(n)
	}

	return base
}
