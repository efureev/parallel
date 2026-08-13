package config

import (
	"fmt"
	"regexp"
	"strings"
)

// placeholderRe находит ссылку на переменную: ${VAR} либо ${VAR:-умолчание}.
//
// Поддерживается только явная форма со скобками. Голый $VAR не раскрывается
// намеренно: в аргументах команд доллар встречается сам по себе — `awk '{print $1}'`,
// `sed 's/$//'`, — и съедать его молча нельзя.
var placeholderRe = regexp.MustCompile(`\$\$?\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// expand подставляет значения переменных в строку.
//
// Отсутствие переменной без умолчания — ошибка с её именем, а не пустая строка:
// пустая строка тихо ломает пути и адреса, и разбираться приходится уже по
// странному поведению запущенной команды.
func expand(s string, lookup map[string]string) (string, error) {
	if !strings.Contains(s, "${") {
		return s, nil
	}

	var missing, nested []string

	result := placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		// $${VAR} — способ написать литеральную ${VAR}: снимаем один доллар
		// и на этом останавливаемся.
		if strings.HasPrefix(match, "$$") {
			return match[1:]
		}

		groups := placeholderRe.FindStringSubmatch(match)
		name, hasDefault, fallback := groups[1], groups[2] != "", groups[3]

		// Вложенная форма: умолчание ограничено первой закрывающей скобкой,
		// поэтому ${A:-${B:-x}} раскрылось бы в «${B:-x}» — строку с недобитой
		// подстановкой, которая уехала бы в аргумент команды буквально.
		// Отказ громче тихо неверного результата.
		if hasDefault && strings.Contains(fallback, "${") {
			nested = append(nested, name)

			return ""
		}

		if value, ok := lookup[name]; ok {
			return value
		}

		if hasDefault {
			return fallback
		}

		missing = append(missing, name)

		return ""
	})

	if len(nested) > 0 {
		return "", fmt.Errorf("%w: %s", ErrNestedPlaceholder, strings.Join(nested, ", "))
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("%w: %s", ErrUndefinedVariable, strings.Join(missing, ", "))
	}

	return result, nil
}

// expandAll подставляет переменные в каждый элемент среза.
func expandAll(items []string, lookup map[string]string) ([]string, error) {
	if len(items) == 0 {
		return items, nil
	}

	out := make([]string, len(items))

	for i, item := range items {
		expanded, err := expand(item, lookup)
		if err != nil {
			return nil, err
		}

		out[i] = expanded
	}

	return out, nil
}
