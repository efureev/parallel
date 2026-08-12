# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-08-13

### Added

- **Конфигурация ищется вверх по дереву каталогов.** Раньше поиск шёл только в текущем
  каталоге, поэтому запуск из подкаталога проекта падал с «config file not found» — при том,
  что файл в проекте есть. Теперь подъём идёт до корня файловой системы, как это делает `git`
  со своей конфигурацией; побеждает ближайший каталог. Относительные значения `dir`
  разрешаются от найденного файла, так что конфигурация из корня проекта работает из любого
  подкаталога.
- **Принимается расширение `.parallelrc.yml`.** Обе формы одинаково распространены, а
  пользователь, назвавший файл `.yml`, получал ошибку с именем, которого он не писал. Внутри
  одного каталога `.yaml` предпочитается `.yml`.
- Сообщение о ненайденной конфигурации перечисляет искомые имена и указывает, что поиск шёл
  по родительским каталогам.

### Changed

- **Путь из `-f` берётся буквально и наверху не ищется.** Опечатка в явном пути обязана
  падать, а не подхватывать чужую конфигурацию из родительского каталога. Значение `-f ""`
  по-прежнему ошибка.

## [1.0.0] - 2026-08-13

Первая мажорная версия. Полная переработка внутреннего устройства при неизменном
поведении командной строки: конфигурации, работавшие на `0.x`, продолжают работать
без правок, а вывод на той же конфигурации совпадает с прежним посимвольно.

**Ломающие изменения**

- **Импорт `github.com/efureev/parallel/src` перестал существовать.** Весь код переехал в
  `cmd/parallel` и `internal/*`, то есть импортируемого Go-API у проекта больше нет вовсе.
  `parallel` — команда, а не библиотека. Установка через `go install` не изменилась.
- **На Windows завершение стало мягким.** Дочерней группе процессов доставляется консольное
  событие `CTRL_BREAK` (его обрабатывают Node.js, Python, .NET и Go), и лишь затем, по таймауту,
  выполняется убийство. Раньше процесс убивался сразу, без шанса закрыть соединения.
- **Повторный Ctrl+C больше не игнорируется.** Первый — вежливая остановка, второй — немедленное
  убийство всех процессов, третий — выход с кодом `130`.

**Замороженный контракт `v1`** — флаги, коды возврата, схема конфигурации и семантика исполнения;
подробности в разделе Compatibility файла `README.md`. Формат строк лога, палитра и тексты ошибок
в контракт не входят и могут меняться. Путь модуля остаётся без суффикса `/vN`.

### Fixed

- **Опечатка в имени поля конфигурации молча игнорировалась.** `pipeline` вместо `pipe`,
  `diir` вместо `dir` — файл принимался, команда выполнялась не так, как задумал автор, и
  узнать об этом было можно только по косвенным признакам. Теперь неизвестное поле — ошибка
  с указанием места и подсказкой ближайшего известного имени.
- **`parallel -h` завершался с кодом 1** и печатал «Failed to parse flags», то есть просьба
  показать справку считалась сбоем и ломала скрипты. Текст справки заодно переписан:
  автогенерируемый показывал `-v` и `-version` двумя отдельными пунктами и не давал примеров.
- **`env` в режиме `docker` не доходил до контейнера.** Переменные задавались процессу клиента
  `docker`, а тот своё окружение контейнеру не передаёт, поэтому написанное рядом с секцией
  `docker` не доходило никуда. Теперь они превращаются в аргументы `-e KEY=VALUE`. Обычные
  команды не затронуты: у них `env` по-прежнему окружение процесса.
- **Код возврата не различал причины отказа.** Любая ошибка схлопывалась в `1`, и скрипт не
  мог отличить «команда упала с кодом 2» от «конфигурация не читается». Теперь наружу уходит
  код команды, чей отказ остановил запуск; ошибки конфигурации и запуска по-прежнему дают `1`.
- **Относительный `dir` разрешался от текущего каталога процесса**, а не от файла
  конфигурации. Конфигурация работала только при запуске из «правильного» места, хотя лежит
  рядом с проектом. Абсолютные пути не затронуты; для конфигурации в корне проекта, запускаемой
  оттуда же, поведение не меняется.
- **Отказ второй и последующих pipe-команд в цепочке терялся:** ожидание группы возвращало
  только первую ошибку. Ошибки собираются по позиции команды, поэтому и состав, и порядок
  теперь не зависят от планировщика.
- **Несуществующий рабочий каталог сообщался как ошибка исполняемого файла** —
  «fork/exec /bin/pwd: no such file or directory», хотя отсутствовал каталог. Теперь так и
  написано: `working directory "…" does not exist`.
- **У каждой piped-команды терялись последние строки вывода.** `cmd.Wait()` закрывает пайпы, как
  только процесс вышел, а чтение вывода отменялось до того, как читатели дочитали остаток. Терялось
  порядка 60 строк на команду независимо от общего объёма — то есть ровно итог работы: результат
  `go test`, сообщение об ошибке перед выходом, хвост сборки. Порядок завершения инвертирован:
  сначала вывод дочитывается до EOF, затем собирается статус процесса. Сторожит
  `TestE2EOutputIntegrity`, проверяющий, что до пользователя дошли все строки до единой.
- **Строки вывода разных цепочек могли накладываться друг на друга.** Запись в общий дескриптор из
  множества горутин не была синхронизирована; на строках длиннее `PIPE_BUF` это давало
  чересполосицу. Записи сериализованы.
- **Утечка горутины в `ExecuteParallel`.** Наблюдатель за отменой ждал родительский контекст и, если
  тот не отменялся, жил до конца процесса.
- **Ошибки цепочек терялись.** Возвращалась первая пришедшая, остальные отбрасывались, причём какая
  именно долетит — зависело от планировщика. Теперь возвращаются все через `errors.Join`.
- **Ошибка разбора конфигурации не указывала место.** Теперь сообщение содержит файл, строку,
  колонку и фрагмент исходника с указателем на проблемное место.
- **`FlowBuilder.Build` при отсутствии ключа `commands` возвращал пустой `Flow`** вместо ошибки.

### Added

- **`env:` в схеме команды** — переменные окружения для конкретной команды. Дополняют окружение
  процесса, а не заменяют его; при совпадении ключей побеждает значение из конфигурации.
- **Флаг `-log-level`** (`debug`, `info`, `warn`, `error`). Отладочные сообщения в коде были,
  но до пользователя не доходили никогда и включить их было нечем — при разборе зависшего
  процесса нужны как раз они.
- **Каталог `examples/`** с двумя самодостаточными конфигурациями, работающими из коробки.
- **Предупреждение о несуществующем рабочем каталоге** при старте. Именно предупреждение, а не
  ошибка: каталог может создаваться предыдущей командой цепочки.
- Бенчмарки горячего пути и e2e-проверка целостности вывода.

### Changed

- **Раскладка проекта приведена к стандартной для Go**: `cmd/parallel` плюс `internal/*` по слоям
  (`flow`, `config`, `runner`, `ui`, `cli`, `buildinfo`). Направление зависимостей между слоями
  проверяется линтером `depguard`, а не соглашением.
- **Логирование изолировано за собственным интерфейсом**: библиотека упоминается ровно в одном файле
  проекта, поэтому смена её мажорной версии больше не задевает остальной код.
- **Зависимости:** `reggol` обновлён с `0.4.1` до `1.2.1`, разбор YAML переведён с
  `gopkg.in/yaml.v3` (без релизов с 2022 года) на `goccy/go-yaml`, добавлены `golang.org/x/sync`
  и `golang.org/x/sys`. Минимальная версия Go поднята до 1.26.
- **Производительность вывода.** Форматирование блочного вывода было квадратичным: на 10 000 строк
  уходило 4.59 ГБ выделенной памяти и треть секунды — теперь 926 КБ и 0.13 мс. Сквозной путь строки
  подешевел с 900 до 272 нс, пропускная способность на реальном пайпе выросла с 268 до 803 тысяч
  строк в секунду. Вывод буферизуется и сбрасывается по таймеру, так что интерактивность сохраняется.
- **Тайминги завершения настраиваются**, а не зашиты константами. Полный прогон тестов ускорился
  с 6.6 до 4.2 секунды, самый долгий тест — с 3.3 до 0.3 секунды.
- `build.sh` даёт имя файла по реальному `GOARCH`: сборка под arm64 больше не выдаёт файл
  с суффиксом `.x64`.

## [0.11.0] - 2026-06-01

### Fixed

- **Command parent now points to the stored chain.** `Flow.Chains` is now
  `[]*CommandChain` and the builder links each command to the very chain object kept
  in the slice (instead of a leaked by-value copy of a loop variable). This removes a
  latent bug where mutating a chain in the slice would diverge from what its commands
  observe via `GetChain()`.
- **`setupPipes` preserves the original cause.** When closing the first pipe fails
  after the second pipe could not be created, both errors are now combined with
  `errors.Join`, so the original "failed creating stderr pipe" cause is no longer lost.

### Changed

- **Sentinel validation errors.** `Command.Validate` / `Flow.Validate` now return
  dedicated sentinel errors (`ErrEmptyCommand`, `ErrNoChains`, `ErrEmptyChainName`,
  `ErrEmptyChain`) instead of ad-hoc `fmt.Errorf` strings, making them testable via
  `errors.Is` and consistent with `manager`'s `ErrCommandExecution` / `ErrPipeCreation`.
- **Unified process supervision.** The near-duplicate orchestration in `Execute` and
  `ExecuteWithPipe` (register → wait → signal → timeout → force-kill) is extracted into
  a shared `manager.supervise` helper with `onCancel`/`onDone` hooks; pipe output
  goroutines are spawned via a small `streamPipes` helper using `wg.Go`. The
  `//nolint:funlen` directives on both methods were removed.

### Added

- Integration tests for the command runner: `Execute`/`ExecuteWithPipe` success and
  failure (exit codes), context cancellation, force-kill of a signal-ignoring process,
  and end-to-end `ExecuteParallel` over a built `Flow` (including failure propagation).
- `TestFlowBuilder_CommandParentPointsToStoredChain` validating the parent-linkage fix,
  plus additional coverage for `FileLoader`, `FlowReader`, version strings and
  `SetShutdownSignal`. Statement coverage of the `src` package rose to ~90%.

[0.11.0]: https://github.com/efureev/parallel/releases/tag/0.11.0

## [0.10.0] - 2026-06-01

### Fixed

- **Deterministic configuration order.** YAML is now parsed via `yaml.Node` (AST)
  instead of nested maps, so the order of chains and of commands inside each chain
  exactly mirrors the configuration file and is stable between runs. This also makes
  color assignment to chains deterministic.
- **Eliminated double / concurrent `cmd.Wait()`.** The executor is now the single
  owner of `cmd.Wait()`: it waits once and closes a `done` channel. The process
  registry stores a `trackedProcess{cmd, done}` and only waits on that channel during
  shutdown, removing the race and the `Wait was already called` error.
- **Correct process exit codes.** `main` now runs through `run() int` + `os.Exit(run())`,
  so the CLI returns a non-zero exit code on failure (missing config, failed command),
  allowing scripts and CI to detect errors.
- **Fixed `PathExists`.** Now returns `err == nil` instead of the masking
  `|| !os.IsNotExist(err)`, which previously reported `true` on errors such as
  permission denied.
- **Fixed Windows test build.** Tests no longer reference the Unix-only
  `syscall.SysProcAttr{Setpgid}` field directly (which broke compilation on Windows);
  they now use the cross-platform `configureProcessGroup` helper. A new
  per-OS test helper `sleepCmdNameArgs` provides a portable long-running command
  (`sleep` on Unix, `ping -n` on Windows), so process/registry tests build and run on
  the `windows-latest` runner.
- **Interruptible output reading.** `handleOutput` no longer blocks on
  `reader.ReadString('\n')` for a silent process: reading runs in a goroutine while
  the main loop selects between new lines, a read error, and context cancellation, so
  output handling is cancelled immediately when the context is done. Also switched the
  EOF check to `errors.Is(err, io.EOF)`.

### Changed

- **Split stdout/stderr.** Non-pipe commands now capture stdout and stderr into
  separate buffers and print them as distinct blocks (`printBlock` / `indentBlock`),
  preserving stream separation.
- **Version output.** The version string is now printed via `fmt.Println` instead of
  `log.Print`, removing the logger timestamp prefix from `-v` output.
- **Cross-platform process-group handling.** Unix-specific process group logic
  (`Setpgid`, `Getpgid`, `Kill(-pgid, …)`) is moved behind build-tagged files
  `process_group_unix.go` (`//go:build !windows`) and `process_group_windows.go`
  (`//go:build windows`), exposing a common API (`configureProcessGroup`,
  `sendSignalToGroup`, `killProcessGroup`). The project now also builds for Windows
  (`CREATE_NEW_PROCESS_GROUP` + process kill fallback).
- **CI updated for cross-platform support.** The build workflow now produces binaries
  for `windows/amd64` (with `.exe` suffix) and `arm64` variants of Linux/macOS in
  addition to the existing `amd64` targets, and bash steps are pinned with
  `shell: bash` so they run correctly on the Windows runner. The test workflow matrix
  now also runs on `windows-latest`.
- **Documentation updated.** `README.md` now states Windows support, documents the
  CLI exit codes (`0`/`1`), adds a platform note about Windows shutdown semantics,
  clarifies that chain/command order is deterministic, and references the `make`
  targets and this changelog.


### Added

- New `TestYamlMarshaller_PreservesOrder` test verifying that chain and command order
  is preserved and deterministic across repeated parses.
- New `TestOutputFormatter_HandleOutputCanceled` test verifying that output reading
  is aborted on context cancellation even when the reader is blocked.

[0.10.0]: https://github.com/efureev/parallel/releases/tag/0.10.0
[1.0.0]: https://github.com/efureev/parallel/releases/tag/v1.0.0
