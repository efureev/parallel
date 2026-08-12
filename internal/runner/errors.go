package runner

import (
	"errors"
	"fmt"
)

var (
	ErrCommandExecution = errors.New("command execution failed")
	ErrPipeCreation     = errors.New("pipe creation failed")
)

// minExitCode и maxExitCode — диапазон кодов, которые имеет смысл передавать
// наружу. Убитый сигналом процесс даёт -1, а коды выше 255 оболочка всё равно
// усечёт, поэтому такие значения к коду возврата утилиты не пробрасываются.
const (
	minExitCode = 1
	maxExitCode = 255
)

// ExitError — команда завершилась с ненулевым кодом.
//
// Код хранится полем, а не только в тексте сообщения: по нему вызывающий слой
// решает, с чем завершиться самой утилите. Раньше любой отказ схлопывался в `1`,
// и скрипт не мог отличить «команда упала с кодом 2» от «конфигурация не читается».
type ExitError struct {
	Chain   string
	Command string
	Code    int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s: command %q in chain %q exited with status %d",
		ErrCommandExecution.Error(), e.Command, e.Chain, e.Code)
}

// Unwrap сохраняет сравнение через errors.Is(err, ErrCommandExecution):
// вызывающий код, которому важен только факт отказа, не должен знать про ExitError.
func (e *ExitError) Unwrap() error { return ErrCommandExecution }

// Usable сообщает, годится ли код для передачи наружу как код возврата утилиты.
func (e *ExitError) Usable() bool {
	return e.Code >= minExitCode && e.Code <= maxExitCode
}

// ExitCode выбирает код возврата для набора ошибок выполнения.
//
// Возвращается код команды, чей отказ остановил запуск. Если отказов уцелело
// несколько — берётся первый в порядке объявления в конфигурации; порядок
// обеспечивают вызывающие, собирая ошибки в слоты по позиции цепочки и команды,
// а не по времени отказа.
//
// Правила «код единственной упавшей команды» здесь быть не может: отказ одной
// цепочки останавливает соседние, и их собственный отказ маскируется отменой.
// Сколько команд «упало» — результат гонки, а не свойство конфигурации, и такое
// правило возвращало бы на одном и том же файле то один код, то другой.
func ExitCode(err error, fallback int) int {
	if err == nil {
		return 0
	}

	if codes := collectExitCodes(err, nil); len(codes) > 0 {
		return codes[0]
	}

	return fallback
}

// collectExitCodes обходит дерево ошибок целиком и собирает все коды выхода.
//
// errors.As здесь не подходит: она сама обходит дерево и возвращает первое
// совпадение, поэтому родительский узел и его потомки дали бы один и тот же код
// несколько раз. А различить «упала одна команда» и «упало три» можно только
// точным подсчётом — ошибки цепочек объединяются через errors.Join.
//
//nolint:errorlint // это обход дерева ошибок, а не сравнение с конкретной ошибкой
func collectExitCodes(err error, acc []int) []int {
	if err == nil {
		return acc
	}

	if exitErr, ok := err.(*ExitError); ok {
		if exitErr.Usable() {
			acc = append(acc, exitErr.Code)
		}

		return acc
	}

	switch u := err.(type) {
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			acc = collectExitCodes(e, acc)
		}
	case interface{ Unwrap() error }:
		acc = collectExitCodes(u.Unwrap(), acc)
	}

	return acc
}
