# Parallel

[![Go Coverage](https://github.com/efureev/parallel/wiki/coverage.svg)](https://raw.githack.com/wiki/efureev/parallel/coverage.html)
[![Test](https://github.com/efureev/parallel/actions/workflows/test.yml/badge.svg)](https://github.com/efureev/parallel/actions/workflows/test.yml)

A small CLI to run multiple console commands in parallel with readable, colored output. Useful for local development when you need to run several services/tools at once (web server, queues, bundlers, watchers, etc.).

Highlights:
- Parallel execution of independent chains; sequential execution inside each chain
- Human‑friendly colored logs per chain with optional streaming (`pipe`)
- Graceful shutdown: forwards the original OS signal to the whole process group and waits
- YAML configuration, Docker helpers, name formatting

This README includes typical use cases and practical examples.

## Installation

Requirements: Go 1.25+ (tested on macOS/Linux)

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

Only one flag is supported currently:
- `-f` — path to YAML config (defaults to `.parallelrc.yaml`)

## Screenshots

![screen1.png](.assets%2Fscreen1.png)
![sceen2.png](.assets%2Fsceen2.png)
![screen3.png](.assets%2Fscreen3.png)

## Configuration (YAML)

Top‑level key: `commands`. It maps chain names to command sets. Each command can be a regular OS command or a Docker recipe.

```yaml
commands: # list of parallel command chains
  php-server:                    # chain name
    artisan:                     # command key inside the chain
      pipe: true                 # stream stdout/stderr
      cmd: ['php', 'artisan', 'serve', '--port', '8010']
      dir: 'app'                 # working directory

  web-services:
    nginx-cmd:
      pipe: true
      cmd: ['docker', 'container', 'run', '--rm', '-p', '8090:80', '--name', 'nginx', 'nginx']
      format:
        cmdName: '%CMD_NAME% %CMD_ARGS%'

  docker-services:               # Docker shorthand mode
    nginx-docker:
      docker:
        image:
          name: 'nginx'
          # tag: 'v1'            # default: 'latest'
          # pull: 'always'       # default: none
        ports: ['127.0.0.1:80:8080', '127.0.0.1:443:8443']
        # removeAfterAll: false  # default: true
        # cmd: 'exec'            # default: 'run'

  frontend:
    list-files:
      cmd: ['ls', '-la']         # executed without pipe
    yarn-dev:
      pipe: true
      cmd: ['yarn', 'dev']
      dir: 'app'

  network:
    ping-test:
      pipe: true
      cmd: ['ping', '-c', '3', 'ya.ru']
```

### Fields

- `pipe: true` — stream output live. If `false`/missing, the output is printed as a block after the command finishes.
- `cmd: ['bin', 'arg1', ...]` — regular command and its args.
- `dir: 'path'` — working directory for the command.
- `format.cmdName` — display name template. Supports placeholders:
  - `%CMD_NAME%` — command name (either `Name` or `Cmd`)
  - `%CMD_ARGS%` — arguments joined by space

### Docker mode

When `docker` section is used, the tool builds the final `docker` command for you, adds `--rm` by default (unless `removeAfterAll: false`), applies `pull` policy and ports, and always runs with `pipe: true` for live logs.

## How it runs

- Parallel starts each chain concurrently.
- Commands inside a chain are executed sequentially, respecting order.
- For `pipe: true`, stdout/stderr are streamed and colorized per chain.
- For non‑pipe commands, output is shown as a formatted block after completion.

## Graceful shutdown

Parallel traps `SIGINT`, `SIGTERM`, `SIGQUIT` and forwards the same signal to the entire process group of each running command (`setpgid` + group signal). Then it waits for completion up to a short timeout and only then force‑kills remaining groups.

What this means for you:
- Press Ctrl+C once to stop everything gracefully.
- Long‑running children that handle signals (e.g., `node`, `php artisan serve`, `yarn`) can clean up before exit.

## Flow preview

Before execution, the tool prints a readable breakdown of your Flow (chains and commands) so you see exactly what will run. Example:

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
parallel -f app/flow.yaml
```

Minimal config to run two commands in parallel:

```yaml
commands:
  api:
    serve:
      pipe: true
      cmd: ['go', 'run', './cmd/api']
  ui:
    dev:
      pipe: true
      cmd: ['yarn', 'dev']
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

## Development

Run tests:

```shell
go test ./...
```

The project structure is split into clear layers:
- file loading (`fileLoader.go`) → raw YAML
- flow building (`flowBuilder.go`) → domain model (`Flow`)
- validation (`Flow.Validate`)
- output formatting (`output.go`, `flowReader.go`)
- execution and shutdown management (`manager.go`, `process_registry.go`, `chain_executor.go`)

## License

MIT

---

Русский кратко

Parallel — утилита для параллельного запуска нескольких команд с читаемым цветным выводом и корректным завершением. Конфигурация — YAML, запуск: `parallel -f app/flow.yaml`. См. примеры выше и `app/flow.yaml`.