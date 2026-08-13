package config

import (
	"fmt"
	"os"
	"strings"
)

// exportPrefix — необязательная приставка, с которой .env-файлы часто пишут,
// чтобы их можно было ещё и `source`-нуть из оболочки.
const exportPrefix = "export "

// inlineCommentSep — начало комментария в хвосте незакавыченного значения.
// Пробел обязателен: без него `PASS=a#b` потерял бы половину пароля.
const inlineCommentSep = " #"

// loadDotEnv читает файл переменных окружения.
//
// Разборщик свой, а не библиотечный: правил здесь на полсотни строк, зато они
// наши и описаны в README, а не «как у той библиотеки».
func loadDotEnv(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrEnvFileRead, path, err)
	}

	env, err := parseDotEnv(string(content))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return env, nil
}

// parseDotEnv разбирает содержимое .env-файла.
func parseDotEnv(content string) (map[string]string, error) {
	env := make(map[string]string)

	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, err := parseDotEnvLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		env[key] = value
	}

	return env, nil
}

// parseDotEnvLine разбирает одну содержательную строку файла.
func parseDotEnvLine(line string) (key, value string, err error) {
	line = strings.TrimPrefix(line, exportPrefix)

	// Делим по первому знаку равенства: он часто встречается и в значениях —
	// в строках подключения, в base64, в аргументах.
	name, rest, found := strings.Cut(line, "=")
	if !found {
		return "", "", fmt.Errorf("%w: %q", ErrEnvFileSyntax, line)
	}

	key = strings.TrimSpace(name)
	if key == "" {
		return "", "", fmt.Errorf("%w: empty variable name", ErrEnvFileSyntax)
	}

	if strings.ContainsAny(key, " \t") {
		return "", "", fmt.Errorf("%w: variable name %q contains spaces", ErrEnvFileSyntax, key)
	}

	return key, dotEnvValue(strings.TrimSpace(rest)), nil
}

// dotEnvValue освобождает значение от кавычек либо отсекает хвостовой комментарий.
//
// Экранирование внутри кавычек не обрабатывается намеренно: частичная поддержка
// escape-последовательностей хуже их явного отсутствия — пользователь не должен
// гадать, какие из них работают.
func dotEnvValue(value string) string {
	const minQuoted = 2

	if len(value) >= minQuoted {
		first, last := value[0], value[len(value)-1]
		if (first == '"' || first == '\'') && first == last {
			return value[1 : len(value)-1]
		}
	}

	// Комментарий отсекается только у незакавыченного значения: в кавычках
	// решётка — обычный символ.
	if idx := strings.Index(value, inlineCommentSep); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}

	return value
}
