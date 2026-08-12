// Package ui отвечает за представление: порт логгера, форматирование вывода
// команд, палитру и человекочитаемый предпросмотр Flow.
package ui

// Порт логгера.
//
// Смысл этого файла — граница. Библиотека логирования упоминается ровно в одном
// месте проекта (logger_reggol.go), а весь остальной код работает с интерфейсом
// Logger. Смена мажорной версии библиотеки или её замена целиком становится
// правкой одного файла вместо восьми.
//
// Интерфейс намеренно узкий: в него попало только то, что проект реально вызывает.

// Field — пара «ключ-значение» для структурированной записи в лог.
type Field struct {
	Key string
	Val any
}

// F собирает поле лога.
func F(key string, val any) Field {
	return Field{Key: key, Val: val}
}

// Logger — контракт логирования, нужный этому проекту.
//
// Blocks и ErrorBlocks обслуживают горячий путь вывода команд: блоки — это
// короткие маркеры перед содержимым (имя цепочки, имя команды).
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(err error, msg string, fields ...Field)

	// Blocks печатает запись без уровня: только блоки, последний из которых — содержимое.
	Blocks(blocks ...string)
	// ErrorBlocks печатает ошибку с предваряющими блоками.
	ErrorBlocks(err error, blocks ...string)
}
