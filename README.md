# Parallel

[![Go Coverage](https://github.com/efureev/parallel/wiki/coverage.svg)](https://raw.githack.com/wiki/efureev/parallel/coverage.html)
[![Test](https://github.com/efureev/parallel/actions/workflows/test.yml/badge.svg)](https://github.com/efureev/parallel/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/efureev/parallel)](https://goreportcard.com/report/github.com/efureev/parallel)

A small CLI to run multiple console commands in parallel with readable, colored output. Useful for local development
when you need to run several services/tools at once (web server, queues, bundlers, watchers, etc.).

Highlights:

- Parallel execution of independent chains
- Inside a chain: non‑piped commands run sequentially; `pipe: true` commands run concurrently within the same chain
- Human‑friendly colored logs per chain with optional streaming (`pipe`)
- Graceful shutdown: forwards the original OS signal to the whole process group and waits
- YAML configuration, Docker helpers, name formatting

This README includes typical use cases and practical examples.

## Installation

Requirements: Go 1.26+ (tested on macOS/Linux/Windows)

```shell
go install github.com/efureev/parallel@latest
```

The binary will be placed at `$(go env GOPATH)/bin/parallel`.

## Quick start

If you have a configuration file `.parallelrc.yaml` in the working directory:

```shell
parallel
```

If the configuration file is located elsewhere:

```shell
parallel -f /path/to/config/flow.yaml
```

Two ready-to-run examples ship with the repository and need no editing:

```shell
parallel -f examples/basic.yaml   # two chains, sequential and streaming
parallel -f examples/full.yaml    # env, dir, disable and the Docker mode
```

Flags are supported currently:

- `-f` — path to YAML config (defaults to `.parallelrc.yaml`)
- `-v` — version info

### Exit codes

Parallel is CI/script friendly and reports a meaningful exit status:

- `0` — all chains finished successfully (also for `-v`).
- `1` — startup/configuration error (e.g., missing or invalid config) or a command in a chain failed.

This lets you safely use `parallel` in scripts and pipelines (e.g., `parallel -f flow.yaml && next-step`).

## Screenshots

![screen1.png](.assets%2Fscreen1.png)
![sceen2.png](.assets%2Fsceen2.png)
![screen3.png](.assets%2Fscreen3.png)

## Configuration (YAML)

Top‑level key: `commands`. It maps chain names to command sets. Each command can be a regular OS command or a Docker
recipe.

```yaml
commands: # list of parallel command chains
  php-server: # chain name
    artisan: # command key inside the chain
      pipe: true                 # stream stdout/stderr
      cmd: [ 'php', 'artisan', 'serve', '--port', '8010' ]
      dir: 'app'                 # working directory

  web-services:
    nginx-cmd:
      pipe: true
      cmd: [ 'docker', 'container', 'run', '--rm', '-p', '8090:80', '--name', 'nginx', 'nginx' ]
      format:
        cmdName: '%CMD_NAME% %CMD_ARGS%'

  docker-services: # Docker shorthand mode
    nginx-docker:
      docker:
        image:
          name: 'nginx'
          # tag: 'v1'            # default: 'latest'
          # pull: 'always'       # default: none
        ports: [ '127.0.0.1:80:8080', '127.0.0.1:443:8443' ]
        # removeAfterAll: false  # default: true
        # cmd: 'exec'            # default: 'run'

  frontend:
    list-files:
      cmd: [ 'ls', '-la' ]         # executed without pipe
    yarn-dev:
      pipe: true
      cmd: [ 'yarn', 'dev' ]
      dir: 'app'

  network:
    ping-test:
      pipe: true
      cmd: [ 'ping', '-c', '3', 'ya.ru' ]
```

### Fields

- `pipe: true` — stream output live and start the command concurrently within its chain. The chain will wait for all
  piped commands to finish before completing.
- `pipe: false` (or missing) — run sequentially, respecting the order in the chain. Output is printed as a block after
  the command finishes.
- `cmd: ['bin', 'arg1', ...]` — regular command and its args.
- `dir: 'path'` — working directory for the command. A path that does not exist produces a warning at startup, not an
  error: the directory may be created by an earlier command in the chain.
- `disable: true` — disable a command without removing it from config. Disabled commands are shown in the flow preview
  and are skipped during execution. Default: `false`.
- `env: { KEY: value }` — environment variables for this command. They are **added to** the environment `parallel`
  itself runs with, not a replacement for it, so there is no need to restate `PATH`. On a key collision the value from
  the config wins. Works for both regular and Docker commands.
- `format.cmdName` — display name template. Supports placeholders:
    - `%CMD_NAME%` — command name (either `Name` or `Cmd`)
    - `%CMD_ARGS%` — arguments joined by space

```yaml
commands:
  api:
    serve:
      pipe: true
      cmd: [ 'go', 'run', './cmd/api' ]
      env:
        APP_ENV: development
        PORT: '8080'
```

### Docker mode

When `docker` section is used, the tool builds the final `docker` command for you, adds `--rm` by default (unless
`removeAfterAll: false`), applies `pull` policy and ports, and always runs with `pipe: true` for live logs. Because of
this, Docker commands start concurrently and the chain waits for them to finish.

Example of disabling commands (works for both regular and docker forms):

```yaml
commands:
  api:
    serve:
      pipe: true
      disable: true           # will be listed but not executed
      cmd: [ 'go', 'run', './cmd/api' ]

  docker-services:
    nginx:
      disable: true           # will be skipped
      docker:
        image:
          name: nginx
```

## How it runs

- Parallel starts each chain concurrently.
- Inside a chain:
    - Commands with `pipe: false` are executed sequentially, in the exact order they appear in the YAML (chain and
      command order is preserved deterministically across runs).
    - Commands with `pipe: true` start immediately and run concurrently with others in the same chain; the chain
      completes only after all piped commands finish.
    - If a non‑piped command fails, subsequent commands in that chain are not started; already running piped commands
      are awaited.
- For `pipe: true`, stdout/stderr are streamed and colorized per chain.
- For non‑pipe commands, output is shown as a formatted block after completion.

Example of mixing piped and non‑piped commands in one chain:

```yaml
commands:
  api:
    migrate:
      cmd: [ 'go', 'run', './cmd/migrate' ] # runs sequentially while long-runner keeps streaming

    long-runner:
      pipe: true
      cmd: [ 'go', 'run', './cmd/api' ]

    health-check:
      pipe: true                          # starts concurrently; chain will wait for it at the end
      cmd: [ 'curl', '-s', 'http://localhost:8080/health' ]
```

## Graceful shutdown

Parallel traps `SIGINT`, `SIGTERM`, `SIGQUIT` and forwards the same signal to the entire process group of each running
command (`setpgid` + group signal). Then it waits for completion up to a short timeout and only then force‑kills
remaining groups.

> Platform note: on Windows arbitrary POSIX signals cannot be delivered. Children are started in a new process group
> (`CREATE_NEW_PROCESS_GROUP`) and receive a `CTRL_BREAK` console event instead, which Node.js, Python, .NET and Go
> runtimes all handle. If a process does not exit after that, it is killed as a fallback — the same two-step ladder
> as on Unix.

What this means for you:

- Press Ctrl+C **once** to stop everything gracefully. Long‑running children that handle signals (`node`,
  `php artisan serve`, `yarn`) get a chance to clean up before exit.
- Press Ctrl+C **twice** to stop waiting and kill every process immediately.
- A third Ctrl+C exits `parallel` itself at once, with status `130`.

Output is never truncated on shutdown: everything a command printed before it exited is read and displayed,
including the last lines produced right before the process died.

## Flow preview

Before execution, the tool prints a readable breakdown of your Flow (chains and commands) so you see exactly what will
run. Example:

```
Flow structure:
  Chain 1: server
    [1] php
        Exec : php artisan queue:work --queue=image-resizing
        Dir  : /path/to/app
        Pipe : true
        Name : %CMD_NAME%
```

## Typical use cases

- Web + Frontend dev:
    - Laravel/Symfony server, queue workers, plus `yarn dev`
    - Vite/webpack dev server together with API
- Micro‑services demo: run several APIs + Nginx proxy in Docker
- Background jobs: watch two queues and a scheduler simultaneously
- Diagnostics: tail logs, run `ping`/`curl`/`watch` side by side

## Examples

Run with the default config in cwd:

```shell
parallel
```

Run with a custom config path:

```shell
parallel -f examples/full.yaml
```

Minimal config to run two commands in parallel:

```yaml
commands:
  api:
    serve:
      pipe: true
      cmd: [ 'go', 'run', './cmd/api' ]
  ui:
    dev:
      pipe: true
      cmd: [ 'yarn', 'dev' ]
      dir: 'web'
```

## Troubleshooting

- Command exits immediately with no output
    - Check `cmd` and arguments; make sure the binary exists in `PATH`
    - Verify `dir` points to a valid folder
- Docker command keeps running after Ctrl+C
    - The tool sends signal to the process group; ensure your containerized process handles `SIGTERM` and stops promptly
- YAML error: “invalid flow configuration”
    - The tool validates that each chain has at least one command and each command has a non‑empty `cmd`

## Compatibility

Starting with `v1.0.0` the following is frozen and will not change without a `v2`:

- **CLI flags** — `-f <path>`, `-v`, `--version`; the default config path `.parallelrc.yaml`.
- **Exit codes** — `0` when every chain succeeded (and for `-v`); `1` on a startup/configuration
  error or a failed command.
- **Configuration schema** — the top-level `commands` key and the command fields `cmd`, `dir`,
  `pipe`, `disable`, `env`, `format.cmdName`, `docker.*`, plus the `%CMD_NAME%` / `%CMD_ARGS%`
  placeholders.
- **Execution semantics** — chains run in parallel; inside a chain non-`pipe` commands run
  sequentially in YAML order, `pipe` commands run concurrently, and the chain waits for all of them.

Deliberately **not** frozen, so it may change in any release: the exact log line format and
ordering, the color palette, the look of the flow preview, and error message wording.

The module path stays `github.com/efureev/parallel` — the `/vN` suffix is only required from `v2`
onwards. There is no importable Go API: everything lives under `internal/`, so `parallel` is a
command, not a library.

## Development

Run tests:

```shell
go test ./...
```

Build the binary:

```shell
go build -o parallel ./cmd/parallel
```

The repository also ships `make` targets (run inside Docker, no local tooling required):

- `make test` — run linters and tests
- `make lint` — run `golangci-lint`
- `make fmt` — format the code (goimports + `gofmt -s` + `go mod tidy`)

See [`CHANGELOG.md`](CHANGELOG.md) for the list of notable changes.

The project follows the standard Go layout, one package per layer:

```
cmd/parallel/        entry point: wiring and the exit code
internal/
  buildinfo/         version metadata injected via -ldflags
  flow/              domain: Flow, CommandChain, Command, validation
  config/            YAML schema, loading, building the domain Flow
  runner/            process supervision, registry, process groups
  ui/                logger port, output rendering, palette, flow preview
  cli/               flags, signals, dependency wiring
```

Dependencies only ever point one way: `cli → {config, flow, runner, ui}`, `runner → {flow, ui}`,
`config → flow`, `ui → flow`, and `flow` depends on nothing at all. The rule is enforced by
`depguard` in `golangci-lint`, not by convention.

## License

MIT

---

Русский кратко

Parallel — утилита для параллельного запуска нескольких команд с читаемым цветным выводом и корректным завершением.
Конфигурация — YAML, запуск: `parallel -f examples/basic.yaml`.

Первый Ctrl+C останавливает всё вежливо, второй — немедленно, третий выходит сразу. Вывод команд при
завершении не теряется: всё, что команда успела напечатать, будет показано. Готовые примеры
конфигурации лежат в каталоге `examples/`.