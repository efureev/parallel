package config

import "errors"

// Сентинельные ошибки слоя конфигурации.
//
// Раньше эти случаи возвращались через fmt.Errorf со свободным текстом и не
// поддавались сравнению через errors.Is, хотя остальной код проекта уже был
// переведён на сентинелы. Это самые вероятные ошибки в реальной
// эксплуатации — неверный путь к конфигу, — поэтому проверяемость здесь важнее
// всего.
var (
	// ErrEmptyConfigPath — путь к файлу конфигурации не задан.
	ErrEmptyConfigPath = errors.New("config file path is empty")
	// ErrConfigNotFound — файл конфигурации не существует или недоступен.
	ErrConfigNotFound = errors.New("config file not found")
	// ErrConfigRead — файл существует, но прочитать его не удалось.
	ErrConfigRead = errors.New("cannot read config file")
	// ErrConfigParse — разборщик YAML упал на этом вводе.
	ErrConfigParse = errors.New("cannot parse config file")
	// ErrConfigDecode — содержимое файла не удалось разобрать.
	ErrConfigDecode = errors.New("cannot decode config file")
	// ErrNoAdHocCommands — команды после `--` не заданы или пусты.
	ErrNoAdHocCommands = errors.New("no commands to run")
	// ErrEnvFileRead — файл переменных окружения не найден или нечитаем.
	ErrEnvFileRead = errors.New("cannot read env file")
	// ErrEnvFileSyntax — строка файла переменных не разбирается.
	ErrEnvFileSyntax = errors.New("malformed env file line")
	// ErrNestedPlaceholder — подстановка внутри умолчания другой подстановки.
	ErrNestedPlaceholder = errors.New("nested substitution is not supported")
	// ErrUndefinedVariable — ссылка на переменную без значения и без умолчания.
	ErrUndefinedVariable = errors.New("undefined variable")
	// ErrNegativeValue — числовое поле получило отрицательное значение.
	ErrNegativeValue = errors.New("value cannot be negative")
	// ErrCmdAndRun — заданы одновременно cmd и run.
	ErrCmdAndRun = errors.New("command cannot use both 'cmd' and 'run'")
	// ErrMissingCommands — в конфигурации нет верхнеуровневого ключа commands.
	ErrMissingCommands = errors.New("config must contain the 'commands' key")
)
