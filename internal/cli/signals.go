package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// shutdownSignals — сигналы, по которым утилита начинает штатное завершение.
//
//nolint:gochecknoglobals // список сигналов неизменен и используется в двух местах
var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

// signalHandler — реакция приложения на сигналы завершения.
type signalHandler struct {
	// onFirst вызывается на первый сигнал: штатная остановка команд.
	onFirst func(os.Signal)
	// onSecond вызывается на второй: немедленное убийство всего.
	onSecond func()
	// exit завершает процесс на третьем сигнале.
	exit func(code int)
}

const (
	// exitCodeInterrupted — код возврата при прерывании пользователем.
	// 130 — общепринятое значение для «завершено по SIGINT» (128 + номер сигнала).
	exitCodeInterrupted = 130

	// Ступени реакции на повторные сигналы.
	signalGraceful = 1 // вежливая остановка
	signalForce    = 2 // немедленное убийство процессов
)

// watchSignals обрабатывает сигналы завершения по нарастающей.
//
// Раньше здесь читался ровно один сигнал, после чего горутина завершалась,
// а последующие копились в буфере канала и не читались никем (находка C3).
// Пользователь, у которого дочерний процесс не реагирует на SIGTERM, не мог
// прервать пятнадцатисекундное ожидание вообще ничем.
//
// Теперь действует привычное для консольных утилит правило: первый Ctrl+C —
// вежливая остановка, второй — немедленная, третий — выход без разговоров.
func watchSignals(ctx context.Context, sigCh <-chan os.Signal, h signalHandler) {
	var count int

	for {
		select {
		case <-ctx.Done():
			return
		case sig, ok := <-sigCh:
			if !ok {
				return
			}

			count++

			switch count {
			case signalGraceful:
				h.onFirst(sig)
			case signalForce:
				h.onSecond()
			default:
				h.exit(exitCodeInterrupted)
			}
		}
	}
}

// notifyShutdown подписывается на сигналы завершения.
//
// Буфер канала рассчитан на всю лестницу реакций: сигналы доставляются
// неблокирующе, и без запаса третий Ctrl+C мог бы потеряться ровно тогда,
// когда он нужнее всего.
func notifyShutdown() (chan os.Signal, func()) {
	sigCh := make(chan os.Signal, len(shutdownSignals))
	signal.Notify(sigCh, shutdownSignals...)

	return sigCh, func() { signal.Stop(sigCh) }
}
