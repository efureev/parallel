package ui

import (
	"io"
	"os"
	"time"

	"github.com/efureev/reggol"
)

// ЕДИНСТВЕННОЕ место в проекте, где известно про библиотеку логирования.
// Всё остальное работает с интерфейсом Logger из logger.go. Правило проверяется
// линтером depguard: ни один пакет вне internal/ui не импортирует reggol.

// reggolLogger реализует Logger поверх reggol.
type reggolLogger struct {
	lgr *reggol.Logger
}

// NewLogger собирает логгер приложения, пишущий в out.
//
// out передаётся явно: от него зависит и назначение вывода, и решение о раскраске.
//
// SyncWriter обязателен: приложение пишет в один дескриптор из множества горутин
// (по две на каждую piped-команду), и без сериализации длинные строки разных
// цепочек накладываются друг на друга.
func NewLogger(out io.Writer, opts ...Option) Logger {
	cfg := newConfig(opts)

	return newLoggerOver(reggol.SyncWriter(out), isTerminal(out), cfg.level)
}

// Option настраивает вывод.
type Option func(*config)

type config struct {
	level Level
}

func newConfig(opts []Option) config {
	cfg := config{level: LevelInfo}
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// WithLevel задаёт минимальный уровень, попадающий в вывод.
func WithLevel(l Level) Option {
	return func(c *config) { c.level = l }
}

// toReggolLevel переводит уровень порта в уровень библиотеки.
func toReggolLevel(l Level) reggol.Level {
	switch l {
	case LevelDebug:
		return reggol.DebugLevel
	case LevelInfo:
		return reggol.InfoLevel
	case LevelWarn:
		return reggol.WarnLevel
	case LevelError:
		return reggol.ErrorLevel
	default:
		return reggol.InfoLevel
	}
}

// newLoggerOver собирает логгер поверх уже потокобезопасного writer.
//
// colored передаётся отдельно, потому что writer к этому моменту может быть
// обёрткой (буфером, sink'ом), по которой терминальность уже не определить.
func newLoggerOver(out io.Writer, colored bool, level Level) Logger {
	mode := reggol.ColorNever
	if colored {
		mode = reggol.ColorAlways
	}

	enc := reggol.NewConsoleEncoder(
		reggol.WithColorMode(mode, out),
		reggol.WithConsoleOptions(reggol.WithTimeFormat(time.TimeOnly)),
	)

	// Глобальный порог библиотеки по умолчанию отсекает всё ниже Info, и без
	// его снятия наш уровень не имел бы силы: событие фильтруется дважды.
	// Значение здесь константа, поэтому глобальное состояние не зависит от флагов —
	// фильтрует уровень конкретного логгера.
	reggol.SetGlobalLevel(reggol.TraceLevel)

	l := reggol.New(out, reggol.WithEncoder(enc), reggol.WithLevel(toReggolLevel(level)))

	return &reggolLogger{lgr: &l}
}

// Output — связка логгера, форматтера и буфера записи для одного назначения вывода.
//
// Всё трое конструируются вместе намеренно. Решение о раскраске должно быть
// общим: логгер без ANSI и форматтер с ANSI дали бы escape-последовательности
// в файле, куда перенаправлен вывод. А буфер обязан быть один — иначе строки
// перемешаются на выходе.
type Output struct {
	logger    Logger
	formatter *OutputFormatter
	sink      *Sink
}

// Logger возвращает логгер приложения.
func (o *Output) Logger() Logger { return o.logger }

// Formatter возвращает форматтер вывода команд.
func (o *Output) Formatter() *OutputFormatter { return o.formatter }

// Close досбрасывает остаток буфера и останавливает фоновый сброс.
//
// Вызвать обязательно: без этого последние строки вывода не дойдут до
// пользователя, оставшись в буфере.
func (o *Output) Close() error { return o.sink.Close() }

// NewOutput собирает вывод поверх указанного writer.
func NewOutput(out io.Writer, opts ...Option) *Output {
	cfg := newConfig(opts)
	sink := NewSink(out)

	// SyncWriter поверх Sink не нужен: Sink сериализует записи сам.
	lgr := newLoggerOver(sink, isTerminal(out), cfg.level)

	return &Output{
		logger:    lgr,
		formatter: newOutputFormatter(lgr, isTerminal(out)),
		sink:      sink,
	}
}

// NewStdoutOutput собирает вывод для стандартного вывода процесса.
func NewStdoutOutput(opts ...Option) *Output {
	return NewOutput(os.Stdout, opts...)
}

// NewDiscardOutput собирает вывод, отбрасывающий записи: нужен тестам и бенчмаркам.
func NewDiscardOutput() *Output {
	return NewOutput(io.Discard)
}

// NewDiscardLogger собирает логгер, отбрасывающий записи.
func NewDiscardLogger() Logger {
	return NewLogger(io.Discard)
}

func (r *reggolLogger) Debug(msg string, fields ...Field) {
	apply(r.lgr.Debug(), fields).Msg(msg)
}

func (r *reggolLogger) Info(msg string, fields ...Field) {
	apply(r.lgr.Info(), fields).Msg(msg)
}

func (r *reggolLogger) Warn(msg string, fields ...Field) {
	apply(r.lgr.Warn(), fields).Msg(msg)
}

func (r *reggolLogger) Error(err error, msg string, fields ...Field) {
	e := r.lgr.Error()
	if err != nil {
		e = e.Err(err)
	}

	apply(e, fields).Msg(msg)
}

func (r *reggolLogger) Blocks(blocks ...string) {
	r.lgr.Log().Blocks(blocks...).Send()
}

func (r *reggolLogger) ErrorBlocks(err error, blocks ...string) {
	r.lgr.Err(err).Blocks(blocks...).Send()
}

// apply переносит поля порта в событие библиотеки.
func apply(e *reggol.Event, fields []Field) *reggol.Event {
	for _, f := range fields {
		switch v := f.Val.(type) {
		case string:
			e = e.Str(f.Key, v)
		case int:
			e = e.Int(f.Key, v)
		case error:
			e = e.AnErr(f.Key, v)
		default:
			e = e.Any(f.Key, v)
		}
	}

	return e
}

// colorMode переводит решение о раскраске в термины библиотеки.
//
// Решение принимается здесь явно, а не оставляется на ColorAuto, потому что
// тот же ответ нужен палитре в OutputFormatter: раскраска должна включаться
// и выключаться в обоих местах одинаково.
func colorMode(out io.Writer) reggol.ColorMode {
	if isTerminal(out) {
		return reggol.ColorAlways
	}

	return reggol.ColorNever
}

// isTerminal сообщает, является ли назначение вывода терминалом.
//
// Ответ нужен и логгеру, и палитре форматтера, поэтому решение принимается один
// раз и передаётся обоим: иначе ANSI-последовательности попадут в файл при
// `parallel -f … > out.txt`.
func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}

	info, err := f.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
