// Package runner выполняет команды: запуск процессов, супервизия, доставка
// сигналов группе и принудительное завершение.
package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

const (
	outputIndentation = "          "

	// pipeReadBufferSize — размер буфера чтения вывода команды.
	//
	// Дефолтные для bufio 4 КиБ означают частые read(2) на болтливом процессе.
	// 64 КиБ снижают их число на порядок; для CLI лишние десятки килобайт
	// на команду несущественны.
	pipeReadBufferSize = 64 * 1024
)

// newlineBytes — перевод строки в байтовом виде для разбора вывода команд.
//
//nolint:gochecknoglobals // неизменяемая константа-литерал, массивом её не объявить
var newlineBytes = []byte(ui.NewlineChar)

// Manager выполняет цепочки команд.
//
// Конструктор возвращает конкретный тип, а не интерфейс: интерфейс, объявленный
// рядом с единственной реализацией, ничего не абстрагирует, зато прячет от
// потребителя остальные методы. Шов для подмены есть там, где он
// нужен на деле, — CommandRunner на стороне chainExecutor.
type Manager struct {
	lgr ui.Logger

	procs *processRegistry
	// shutdownSig хранит сигнал завершения. Раньше это значение защищал
	// RWMutex, хотя пишется оно максимум один раз за жизнь процесса (P9).
	shutdownSig atomic.Value

	output   *ui.OutputFormatter
	chains   *chainExecutor
	timeouts Timeouts

	// keepGoing переносится в chainExecutor при сборке: опции применяются
	// до его создания, поэтому передать значение напрямую нельзя.
	keepGoing bool

	// commandLimit — предел на команду, заданный флагом -timeout. Поле
	// timeout у самой команды сильнее.
	commandLimit time.Duration

	// maxParallel переносится в chainExecutor при сборке, как и keepGoing.
	maxParallel int
}

// Option настраивает менеджер при создании.
type Option func(*Manager)

// WithTimeouts задаёт тайминги завершения процессов.
// Нужен тестам, чтобы не ждать секундами то, что проверяется миллисекундами.
// WithKeepGoing отключает остановку соседних цепочек при отказе одной из них.
//
// Влияет только на цепочки между собой: внутри цепочки упавшая не-pipe команда
// по-прежнему обрывает остаток — команды в ней обычно зависимы (migrate → serve),
// и продолжать после отказа значило бы работать с заведомо неверным состоянием.
func WithKeepGoing() Option {
	return func(m *Manager) { m.keepGoing = true }
}

// WithCommandTimeout задаёт предел выполнения для всех команд сразу.
//
// Не путать с WithTimeouts: та настраивает лестницу остановки уже снимаемого
// процесса, а эта решает, когда его вообще пора снимать. Поле timeout
// у команды перекрывает это значение.
func WithCommandTimeout(d time.Duration) Option {
	return func(m *Manager) { m.commandLimit = d }
}

// WithMaxParallel ограничивает число одновременно работающих цепочек.
func WithMaxParallel(n int) Option {
	return func(m *Manager) { m.maxParallel = n }
}

func WithTimeouts(t Timeouts) Option {
	return func(m *Manager) { m.timeouts = t.normalize() }
}

// NewManager создаёт новый экземпляр менеджера.
func NewManager(logger ui.Logger, formatter *ui.OutputFormatter, opts ...Option) *Manager {
	m := &Manager{
		lgr:      logger,
		procs:    newProcessRegistry(),
		output:   formatter,
		timeouts: DefaultTimeouts(),
	}

	m.shutdownSig.Store(defaultShutdownSignal())

	for _, opt := range opts {
		opt(m)
	}

	var chainOpts []chainOption
	if m.keepGoing {
		chainOpts = append(chainOpts, withKeepGoing())
	}

	if m.maxParallel > 0 {
		chainOpts = append(chainOpts, withMaxParallel(m.maxParallel))
	}

	m.chains = newChainExecutor(logger, m, m.stopAllCommands, chainOpts...)

	return m
}

// SetShutdownSignal задаёт сигнал, который будет отправляться дочерним процессам
// при завершении работы приложения (например, SIGINT / SIGTERM / SIGQUIT).
//
// Принимает os.Signal, а не syscall.Signal: платформенный тип в кросс-платформенном
// API — протечка. Перевод в платформенное представление выполняют
// build-tagged файлы, знающие, что на их платформе вообще доставимо.
func (m *Manager) SetShutdownSignal(sig os.Signal) {
	if sig == nil {
		return
	}

	m.shutdownSig.Store(sig)
}

func (m *Manager) getShutdownSignal() os.Signal {
	if v, ok := m.shutdownSig.Load().(os.Signal); ok && v != nil {
		return v
	}

	return defaultShutdownSignal()
}

func (m *Manager) stopAllCommands() {
	if m.procs == nil {
		return
	}

	m.procs.stopAll(m.lgr, m.getShutdownSignal(), m.timeouts.ForceKill)
}

// KillAll немедленно убивает все запущенные группы процессов, не давая им
// времени на завершение.
//
// Это реакция на повторный сигнал от пользователя: первый Ctrl+C просит
// остановиться вежливо, второй означает «немедленно». Без него прервать
// ожидание было нечем.
func (m *Manager) KillAll() {
	if m.procs == nil {
		return
	}

	m.procs.killAll(m.lgr)
}

// supervise регистрирует процесс в registry, ждёт его полного завершения либо
// отмены контекста, отправляет сигнал завершения группе и принудительно убивает
// её по таймауту.
//
// waitFn описывает, что значит «команда завершилась полностью». Для команды с
// пайпами это «дочитать вывод до EOF, затем cmd.Wait()», и порядок здесь
// принципиален: cmd.Wait() закрывает пайпы, как только процесс вышел, поэтому
// вызывать его до окончания чтения — значит терять хвост вывода. Документация
// os/exec говорит об этом прямо: «it is incorrect to call Wait before all reads
// from the pipe have completed». Именно на этом терялись последние ~60 строк
// вывода каждой piped-команды.
//
// abandonOutput — аварийный выход: принудительно прекращает чтение, если после
// убийства группы EOF так и не пришёл (открытый конец пайпа мог остаться у
// отцепившегося внука). Штатным способом завершения он больше не является.
//
// Владельцем ожидания является именно эта функция: результат отдаётся через
// канал, а waitDone закрывается ровно один раз; registry лишь ждёт waitDone.
func (m *Manager) supervise(
	ctx context.Context,
	cmd *exec.Cmd,
	chainName string,
	command flow.Command,
	waitFn func() error,
	abandonOutput func(),
) error {
	cmdKey := uniqueCmdKey(chainName, command, cmd.Process.Pid)

	waitErr := make(chan error, 1)
	waitDone := make(chan struct{})

	m.procs.add(cmdKey, cmd, waitDone)
	defer m.procs.remove(cmdKey)

	go func() {
		waitErr <- waitFn()

		close(waitDone)
	}()

	select {
	case <-ctx.Done():
		m.stopCommand(cmd, command, waitDone, abandonOutput)

		return ctx.Err()

	case <-waitDone:
		return <-waitErr
	}
}

// stopCommand останавливает команду по отмене контекста: сигнал группе, затем
// убийство по таймауту, затем — в крайнем случае — отказ от чтения вывода.
func (m *Manager) stopCommand(
	cmd *exec.Cmd,
	command flow.Command,
	waitDone <-chan struct{},
	abandonOutput func(),
) {
	m.lgr.Info("Context canceled, stopping command", ui.F("cmd", command.Cmd))

	if err := sendSignalToGroup(cmd, m.getShutdownSignal()); err != nil {
		m.lgr.Warn("Failed to send shutdown signal to process group", ui.F("err", err), ui.F("cmd", command.Cmd))
	}

	select {
	case <-waitDone:
		return
	case <-time.After(m.timeouts.ForceKill):
	}

	m.lgr.Warn("Force killing command group", ui.F("cmd", command.Cmd))

	if err := killProcessGroup(cmd); err != nil {
		m.lgr.Warn("Failed to kill process group", ui.F("err", err), ui.F("cmd", command.Cmd))
	}

	// Процесс убит, но вывод дочитывается: даём на это ограниченное время,
	// чтобы не потерять последние строки уже мёртвой команды.
	select {
	case <-waitDone:
		return
	case <-time.After(m.timeouts.Drain):
	}

	// EOF не пришёл: открытый конец пайпа удерживает кто-то, кого убийство
	// группы не задело. Дальше ждать нечего.
	m.lgr.Warn("Giving up on reading command output", ui.F("cmd", command.Cmd))

	if abandonOutput != nil {
		abandonOutput()
	}

	<-waitDone
}

// commandTimeout возвращает предел для конкретной команды: собственный, если
// задан, иначе общий из флага. Ноль означает «без предела».
func (m *Manager) commandTimeout(command flow.Command) time.Duration {
	if command.Timeout > 0 {
		return command.Timeout
	}

	return m.commandLimit
}

// withCommandDeadline ограничивает время выполнения команды.
//
// Именно контекст, а не собственный таймер с Kill: отмена запускает ту же
// лестницу остановки, что и Ctrl+C, — сигнал группе, затем убийство, затем
// ограниченное ожидание дочитывания вывода. Прямое убийство потеряло бы хвост
// вывода, а он и объясняет, на чём команда встала.
func (m *Manager) withCommandDeadline(
	ctx context.Context, command flow.Command,
) (context.Context, context.CancelFunc, time.Duration) {
	limit := m.commandTimeout(command)
	if limit <= 0 {
		return ctx, func() {}, 0
	}

	runCtx, cancel := context.WithTimeout(ctx, limit)

	return runCtx, cancel, limit
}

// stopError решает, чем была остановка команды: исчерпанным пределом, отменой
// снаружи или отказом самой команды.
//
// Различие берётся из контекста команды, а не из текста ошибки: дедлайн даёт
// DeadlineExceeded, отмена родителя — Canceled, и перепутать их нельзя.
func (m *Manager) stopError(
	runCtx context.Context, chain *flow.CommandChain, command flow.Command, limit time.Duration, err error,
) error {
	if limit > 0 && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return &TimeoutError{
			Chain:   chainName(chain),
			Command: command.DisplayName(),
			Limit:   limit,
		}
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	return m.completionError(chain, command, err)
}

func (m *Manager) Execute(ctx context.Context, chain *flow.CommandChain, command flow.Command) error {
	//nolint:gosec // command/args come from trusted config for CLI tool
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	setEnv(cmd, command)
	configureProcessGroup(cmd)

	var stdoutBuf, stderrBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		startErr := startError(command, err)
		m.lgr.Error(startErr, "Failed to start command")

		return startErr
	}

	m.lgr.Info("Command started: " + ui.FullDisplayName(chainName(chain), command))

	runCtx, cancel, limit := m.withCommandDeadline(ctx, command)
	defer cancel()

	// Вывод не-pipe команды собирается в буферы самим exec.Cmd, поэтому здесь
	// «завершиться полностью» и «дождаться процесса» — одно и то же.
	if err := m.supervise(runCtx, cmd, chainName(chain), command, cmd.Wait, nil); err != nil {
		// Вывод печатается и при снятии по таймауту: он и объясняет, на чём
		// команда встала, а молчащий отказ не объясняет ничего.
		m.printBlock(chain, command, stdoutBuf.Bytes(), stderrBuf.Bytes())

		return m.stopError(runCtx, chain, command, limit, err)
	}

	m.printBlock(chain, command, stdoutBuf.Bytes(), stderrBuf.Bytes())

	return nil
}

// printBlock печатает результат не-pipe команды, сохраняя разделение потоков:
// stdout выводится обычным блоком, непустой stderr — отдельным блоком ошибки.
func (m *Manager) printBlock(chain *flow.CommandChain, command flow.Command, stdout, stderr []byte) {
	output := m.output.FormatChainInfo(chain, command)

	if len(stdout) > 0 {
		m.lgr.Blocks(output.Header, output.CmdName, indentBlock(stdout))
	}

	if len(stderr) > 0 {
		m.lgr.ErrorBlocks(errors.New(indentBlock(stderr)), output.Header, output.CmdName)
	}
}

// indentBlock форматирует многострочный вывод с отступом для читаемости.
//
// Раньше строка накапливалась конкатенацией в цикле, то есть на каждой итерации
// выделялась заново и копировалась целиком. Поведение было квадратичным: на
// 10 000 строк уходило 4.59 ГБ выделенной памяти и треть секунды при 810 КБ
// входного текста. Здесь один буфер нужного размера и ни одной
// промежуточной строки: len(b) уже не копируется в string, а строки берутся
// подслайсами.
func indentBlock(b []byte) string {
	lineCount := bytes.Count(b, newlineBytes) + 1

	var sb strings.Builder

	// Точная ёмкость: сам текст + отступ и перевод строки на каждую строку
	// + ведущий перевод строки.
	sb.Grow(len(b) + lineCount*(len(outputIndentation)+1) + 1)
	sb.WriteString(ui.NewlineChar)

	for line := range bytes.SplitSeq(b, newlineBytes) {
		sb.WriteString(outputIndentation)
		sb.Write(line)
		sb.WriteString(ui.NewlineChar)
	}

	return sb.String()
}

func (m *Manager) ExecuteWithPipe(ctx context.Context, chain *flow.CommandChain, command flow.Command) error {
	//nolint:gosec // command/args come from trusted config for CLI tool
	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Dir = command.Dir
	setEnv(cmd, command)

	// Настраиваем process group для правильной передачи сигналов
	configureProcessGroup(cmd)

	stdout, stderr, err := setupPipes(cmd)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPipeCreation, err)
	}
	defer stdout.Close()
	defer stderr.Close()

	if err := cmd.Start(); err != nil {
		startErr := startError(command, err)
		m.lgr.Error(startErr, "Failed starting command")

		return startErr
	}

	m.lgr.Info("Command started: " + ui.FullDisplayName(chainName(chain), command))

	// Контекст чтения вывода намеренно НЕ производный от ctx: отмена ctx
	// означает «останови команду», а не «перестань читать её вывод». Чтение
	// прекращается само по EOF, когда процесс закрывает пайпы.
	outputCtx, cancelOutput := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelOutput()

	// Аварийная остановка чтения — закрытие пайпов. Это разблокирует читающий
	// ReadString напрямую, без горутины-посредника и канала на каждую строку.
	// Штатным путём завершения не является: см. supervise.
	abandonOutput := func() {
		cancelOutput()

		_ = stdout.Close()
		_ = stderr.Close()
	}

	wg := m.streamPipes(outputCtx, chain, command, stdout, stderr)

	runCtx, cancel, limit := m.withCommandDeadline(ctx, command)
	defer cancel()

	// «Завершиться полностью» для piped-команды — это сначала дочитать вывод
	// до EOF, и только потом собрать статус процесса.
	waitFn := func() error {
		wg.Wait()

		return cmd.Wait()
	}

	if err := m.supervise(runCtx, cmd, chainName(chain), command, waitFn, abandonOutput); err != nil {
		return m.stopError(runCtx, chain, command, limit, err)
	}

	return nil
}

// streamPipes запускает две горутины чтения stdout/stderr и возвращает WaitGroup,
// по которой можно дождаться завершения обработки вывода.
func (m *Manager) streamPipes(
	ctx context.Context,
	chain *flow.CommandChain,
	command flow.Command,
	stdout, stderr io.ReadCloser,
) *sync.WaitGroup {
	var wg sync.WaitGroup

	// Разделитель неизменен, поэтому берётся готовым: раньше он собирался
	// заново на каждую строку вывода.
	div := m.output.Divider()

	// Условие готовности вида logLine ждёт появления подстроки в выводе, и
	// единственное место, где строка уже собрана, — здесь. Наблюдатель дешёвый:
	// без запуска он выходит по nil-указателю.
	name := chainName(chain)

	stdoutHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		m.chains.observeLine(name, content)

		cmdNameStyled := fmt.Sprintf(`%s (%d) %s`, cmdName, counter, div)
		m.lgr.Blocks(chainNameStyleText, cmdNameStyled, content)
	}

	stderrHandler := func(chainNameStyleText, cmdName, content string, counter int) {
		// Готовность часто печатается именно в stderr — туда пишут журналы
		// многие серверы.
		m.chains.observeLine(name, content)

		m.lgr.ErrorBlocks(errors.New(content), chainNameStyleText, cmdName)
	}

	wg.Go(func() {
		reader := bufio.NewReaderSize(stdout, pipeReadBufferSize)
		if err := m.output.HandleOutput(ctx, reader, chain, command, stdoutHandler); err != nil {
			m.lgr.Error(err, "command failed")
		}
	})

	wg.Go(func() {
		reader := bufio.NewReaderSize(stderr, pipeReadBufferSize)
		if err := m.output.HandleOutput(ctx, reader, chain, command, stderrHandler); err != nil {
			m.lgr.Error(err, "command failed")
		}
	})

	return &wg
}

// startError объясняет отказ запуска.
//
// Несуществующий рабочий каталог операционная система сообщает как ошибку на
// исполняемом файле: «fork/exec /bin/pwd: no such file or directory», хотя
// отсутствует каталог, а не бинарник. Проверяем каталог и говорим прямо.
func startError(command flow.Command, err error) error {
	if command.Dir != "" && !flow.PathExists(command.Dir) {
		return fmt.Errorf("%w: working directory %q does not exist", ErrCommandExecution, command.Dir)
	}

	return fmt.Errorf("%w: %w", ErrCommandExecution, err)
}

// completionError переводит отказ команды в ошибку с кодом выхода.
func (m *Manager) completionError(chain *flow.CommandChain, command flow.Command, waitErr error) error {
	// ExitCode() вместо syscall.WaitStatus: портируемо и не тянет платформенный
	// пакет в кросс-платформенный файл.
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		code := exitErr.ExitCode()
		m.lgr.Error(nil, "Command failed", ui.F("Exit Status", code))

		return &ExitError{Chain: chainName(chain), Command: command.DisplayName(), Code: code}
	}

	m.lgr.Error(waitErr, "command failed")

	return fmt.Errorf("%w: %w", ErrCommandExecution, waitErr)
}

func (m *Manager) ExecuteParallel(ctx context.Context, chains []*flow.CommandChain) error {
	return m.chains.ExecuteParallel(ctx, chains)
}

// Results возвращает исход каждой цепочки последнего запуска.
//
// Осмысленно только после возврата ExecuteParallel: до этого момента срез пуст.
// Отдельный метод, а не второе возвращаемое значение, чтобы не переписывать
// каждый вызов ради данных, которые нужны одному вызывающему из дюжины.
func (m *Manager) Results() []ChainResult {
	return m.chains.results
}

// setEnv задаёт окружение дочернего процесса.
//
// Если у команды нет собственных переменных, Env не трогаем вовсе: exec.Cmd
// с нулевым Env наследует окружение родителя, а прежний безусловный
// os.Environ() копировал весь блок на каждый запуск впустую.
// Собственные переменные дописываются в конец — при совпадении ключей
// побеждает последнее значение, то есть заданное в конфигурации.
func setEnv(cmd *exec.Cmd, command flow.Command) {
	if len(command.Env) == 0 {
		return
	}

	cmd.Env = slices.Concat(os.Environ(), command.Env)
}

// chainName безопасно достаёт имя цепочки.
func chainName(chain *flow.CommandChain) string {
	if chain == nil {
		return ""
	}

	return chain.Name
}

// uniqueCmdKey строит уникальный ключ для реестра процессов: имя цепочки,
// имя команды и PID, чтобы исключить коллизии.
func uniqueCmdKey(chain string, cmd flow.Command, pid int) string {
	base := cmd.DisplayName()

	if chain != "" {
		base = chain + "/" + base
	}

	return fmt.Sprintf("%s_%d", base, pid)
}

func setupPipes(cmd *exec.Cmd) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating stdout pipe: %w", err)
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		// Clean up the first pipe if the second fails, but preserve the original cause.
		if closeErr := stdout.Close(); closeErr != nil {
			return nil, nil, errors.Join(fmt.Errorf("failed creating stderr pipe: %w", err), closeErr)
		}

		return nil, nil, fmt.Errorf("failed creating stderr pipe: %w", err)
	}

	return stdout, stderr, nil
}
