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
	// ErrConfigDecode — содержимое файла не удалось разобрать.
	ErrConfigDecode = errors.New("cannot decode config file")
	// ErrNoAdHocCommands — команды после `--` не заданы или пусты.
	ErrNoAdHocCommands = errors.New("no commands to run")
	// ErrCmdAndRun — заданы одновременно cmd и run.
	ErrCmdAndRun = errors.New("command cannot use both 'cmd' and 'run'")
	// ErrMissingCommands — в конфигурации нет верхнеуровневого ключа commands.
	ErrMissingCommands = errors.New("config must contain the 'commands' key")
)
