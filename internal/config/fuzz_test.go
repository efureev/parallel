package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Фаззинг разбора конфигурации.
//
// Это единственное место, куда попадают недоверенные данные: YAML, файл
// переменных окружения и подстановка `${VAR}` с собственным разбором скобок
// и умолчаний. Все три цели — чистые функции разбора: ни одна не запускает
// процессов и не трогает файловую систему, поэтому фаззить их безопасно.
//
// Проверяется не только отсутствие паник, но и инварианты, которые разборщики
// обещают вызывающему: нарушение любого из них означает настоящий дефект,
// а не просто необычный вход.

// seedFromExamples добавляет к затравке готовые конфигурации из examples/.
//
// Фаззер начинает с осмысленных входов, а не со случайного мусора: до первой
// валидной YAML-структуры он иначе добирался бы очень долго.
func seedFromExamples(f *testing.F) {
	f.Helper()

	matches, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.yaml"))
	if err != nil {
		return
	}

	for _, path := range matches {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}

		f.Add(string(content))
	}
}

// FuzzUnmarshal — разбор YAML-конфигурации.
func FuzzUnmarshal(f *testing.F) {
	seedFromExamples(f)

	f.Add("commands:\n  c:\n    t:\n      cmd: [ 'echo' ]\n")
	f.Add("commands:\n  c:\n    needs: [ other ]\n    t:\n      run: 'echo hi'\n")
	f.Add("failFast: false\nmaxParallel: 2\nenvFile: .env\ncommands:\n  c:\n    t:\n      cmd: [ 'x' ]\n")
	f.Add("commands:\n  c:\n    t:\n      docker:\n        image:\n          name: n\n        args: [ 'a' ]\n")
	f.Add("commands:\n  c:\n    t:\n      cmd: [ 'x' ]\n      ready:\n        tcp: '127.0.0.1:1'\n")
	f.Add("")
	f.Add("commands:")
	f.Add("- not a mapping")

	f.Fuzz(func(t *testing.T, raw string) {
		data, err := YamlFileMarshaller{}.Unmarshal([]byte(raw))
		if err != nil {
			return
		}

		// Разбор обещает две вещи: имена цепочек и команд не пусты, и порядок
		// повторяет файл. Первое проверяемо здесь, второе — отдельным тестом.
		for _, chain := range data.Chains {
			if chain.Name == "" {
				t.Fatalf("цепочка без имени при успешном разборе: %q", raw)
			}

			for _, cmd := range chain.Commands {
				if cmd.Name == "" {
					t.Fatalf("команда без имени в цепочке %q: %q", chain.Name, raw)
				}

				// needs — зарезервированный ключ, он обязан уходить
				// в зависимости, а не в список команд.
				if cmd.Name == needsKey {
					t.Fatalf("зарезервированный ключ попал в команды: %q", raw)
				}
			}
		}

		if data.MaxParallel < 0 {
			t.Fatalf("отрицательный maxParallel принят: %d", data.MaxParallel)
		}
	})
}

// FuzzParseDotEnv — разбор файла переменных окружения.
func FuzzParseDotEnv(f *testing.F) {
	f.Add("KEY=value\n")
	f.Add("# comment\n\nexport A=1\nB=\"two words\"\nC='single'\n")
	f.Add("URL=postgres://host:5432/db?a=1&b=2\n")
	f.Add("EMPTY=\nSPACED  =  trimmed  \n")
	f.Add("PASS=a#b\nNOTE=value # comment\n")
	f.Add("=novalue\n")
	f.Add("no equals sign\n")
	f.Add("\r\nCRLF=yes\r\n")

	f.Fuzz(func(t *testing.T, raw string) {
		env, err := parseDotEnv(raw)
		if err != nil {
			return
		}

		// Инварианты, которые обещает parseDotEnvLine: имя непусто и не
		// содержит пробелов. Иначе такая переменная не дойдёт до процесса —
		// exec молча отбросит некорректную пару.
		for key := range env {
			if key == "" {
				t.Fatalf("пустое имя переменной при успешном разборе: %q", raw)
			}

			if strings.ContainsAny(key, " \t") {
				t.Fatalf("имя переменной с пробелом: %q из %q", key, raw)
			}

			if strings.Contains(key, "=") {
				t.Fatalf("знак равенства в имени переменной: %q из %q", key, raw)
			}
		}
	})
}

// FuzzExpand — подстановка переменных.
func FuzzExpand(f *testing.F) {
	f.Add("plain text")
	f.Add("http://localhost:${PORT}")
	f.Add("${MISSING:-fallback}")
	f.Add("${A}-${B}-${A}")
	f.Add("$${LITERAL}")
	f.Add("awk '{print $1}'")
	f.Add("${A:-${B:-nested}}")
	f.Add("${}")
	f.Add("${UNCLOSED")
	f.Add("${A:-}")

	lookup := map[string]string{"PORT": "8080", "A": "first", "B": "", "EMPTY": ""}

	f.Fuzz(func(t *testing.T, raw string) {
		got, err := expand(raw, lookup)
		if err != nil {
			return
		}

		// Свойства «все ссылки подставлены» здесь нет намеренно, и это не
		// упущение. Подстановка внутреннего вхождения может породить строку,
		// которая сама выглядит ссылкой: `${B${B}}` при пустом B даёт `${B}`,
		// хотя во входе это была другая, вложенная запись. Нераспознанные формы
		// проходят насквозь по той же причине — `${#arr[@]}` и `${x%%.*}`
		// законны в элементах cmd как синтаксис оболочки, и трогать их нельзя.
		//
		// Точные ожидания раскрытия проверяются табличным TestExpand на заведомо
		// корректных входах. Задача фаззинга здесь — паники и детерминированность.
		_ = got

		// Подстановка обязана быть детерминированной: тот же вход и тот же
		// набор значений — тот же результат.
		again, againErr := expand(raw, lookup)
		if againErr != nil || again != got {
			t.Fatalf("недетерминированно: %q и %q (%v) из %q", got, again, againErr, raw)
		}
	})
}
