package runner

import "time"

// ChainResult — итог одной цепочки.
//
// Err хранится как есть, а не сводится к булеву признаку: вызывающему нужен
// и текст причины для сводки, и код возврата, который извлекает ExitCode.
type ChainResult struct {
	Name     string
	Err      error
	Duration time.Duration
	// Stopped — цепочка не доработала: её остановил отказ соседней либо сигнал.
	Stopped bool
}

// Failed сообщает, завершилась ли цепочка отказом.
func (r ChainResult) Failed() bool { return r.Err != nil }
