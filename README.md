# Parallel

[![Go Coverage](https://github.com/efureev/parallel/wiki/coverage.svg)](https://raw.githack.com/wiki/efureev/parallel/coverage.html)
[![Test](https://github.com/efureev/parallel/actions/workflows/test.yml/badge.svg)](https://github.com/efureev/parallel/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/efureev/parallel)](https://goreportcard.com/report/github.com/efureev/parallel)

*Читать по-русски: [README.ru.md](README.ru.md).*

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

If you have a configuration file `.parallelrc.yaml` (or `.parallelrc.yml`) in the working
directory — or in any directory above it:

```shell
parallel
```

The lookup walks up the directory tree the way `git` finds its config, so a config committed at
the project root works from any subdirectory. The nearest directory wins, and `.yaml` is
preferred over `.yml` within the same directory. Relative `dir` values are resolved against the
file that was found, so the whole configuration stays position-independent.

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

- `-f` — path to YAML config; if omitted, `.parallelrc.yaml` / `.parallelrc.yml` is looked up
  in the current directory and every parent
- `-except <names>` — comma-separated chains to skip
- `-list` — list the chains the configuration defines, then exit
- `-dry-run` — show exactly what would run, then exit without starting anything
- `-keep-going` — do not stop the other chains when one of them fails
- `-timeout <duration>` — stop any command running longer than this (e.g. `30s`, `5m`)
- `-no-color` — disable colored output
- `-log-level` — `debug`, `info` (default), `warn` or `error`
- `-v`, `--version` — version info
- `-h`, `--help` — usage

Positional arguments select chains, and `--` switches to running commands with no config at all:

```shell
parallel api ui                              # run only these chains
parallel -except worker                      # everything but this one
parallel -list                               # what does this configuration define?
parallel -dry-run api                        # what exactly would `parallel api` run?
parallel -keep-going                          # report every failure, not just the first
parallel -timeout 5m                          # no command may run longer than five minutes
parallel -- 'go run ./cmd/api' 'yarn dev'    # no configuration file at all
```

Colors follow the [NO_COLOR](https://no-color.org) convention: setting `NO_COLOR` disables them,
`FORCE_COLOR` enables them when the output is not a terminal, and `-no-color` overrides both.

Unknown fields in the configuration are rejected with the exact position and a suggestion:
a typo like `pipeline:` instead of `pipe:` fails loudly instead of silently changing behaviour.

### Exit codes

Parallel is CI/script friendly and reports a meaningful exit status:

- `0` — all chains finished successfully (also for `-v` and `-h`).
- `1` — startup or configuration error: missing file, unknown field, invalid flag.
- `124` — a command was stopped because it exceeded its `timeout`. The value follows the
  convention of `timeout(1)`: a stopped process has no exit status of its own, so one has to be
  chosen.
- *the command's own exit code* — when a command fails, its status is passed through, so
  `parallel -f flow.yaml || echo $?` reports what actually happened. If several commands fail,
  the first one in configuration order wins.

This lets you safely use `parallel` in scripts and pipelines (e.g., `parallel -f flow.yaml && next-step`).

### When one chain fails

By default the failure of a chain stops the others: with a dev stack there is little point in
keeping the frontend up once the API is gone. In CI the opposite is usually wanted — running the
linter, the tests and the build as three chains, you want to see every failure at once rather
than the first one and then two more runs. That is `-keep-going`, or `failFast: false` in the
configuration:

```yaml
failFast: false     # top-level key, next to `commands`
commands:
  lint: ...
  test: ...
```

An explicit flag beats the file, so `-keep-going=false` forces the default behaviour back for a
single run.

`-keep-going` only affects chains relative to each other. Inside a chain nothing changes: a
failed non-`pipe` command still skips the rest of *its* chain, because commands there are
usually dependent — running `serve` after `migrate` failed would just work against a broken
state.

> One consequence worth knowing: with `-keep-going` a failing chain no longer cuts short a
> long-running sibling. A run that includes a dev server will therefore not finish on its own —
> previously the cancellation killed it.

### Summary

When a run involves more than one chain, a summary is printed at the end — the run is over, the
output of five chains is interleaved, and this answers "so which one broke?" without scrolling:

```
Summary:
  api     ok       1.2s
  worker  failed   0.3s  command execution failed: command "bad" in chain "worker" exited with status 42
  ui      stopped  0.3s
```

A command being restarted has not failed yet, so with the default fail-fast the sibling chains
keep running while the attempts last.

> **`restart: always` without a limit means the run never ends by itself.** For a dev stack that
> is exactly right — the server comes back up and you keep working until Ctrl+C. In CI it is a
> trap: set `restartAttempts` there.

`stopped` means the chain did not fail on its own — it was cut short, either by a sibling chain
failing or by Ctrl+C. `timed out` means a command exceeded its limit and was stopped.

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
- `cmd: ['bin', 'arg1', ...]` — regular command and its args. Taken literally: no shell is
  involved, so `&&`, pipes and globs are not interpreted.
- `run: 'npm run dev'` — the same command as a single line, executed through the shell
  (`sh -c` on Unix, `%COMSPEC% /c` on Windows). Use it when you want `&&`, a pipe, a glob or
  variable expansion. Mutually exclusive with `cmd` — specifying both is an error rather than a
  silent preference for one of them.
- `dir: 'path'` — working directory for the command. Relative paths are resolved against the
  **configuration file**, not the current directory, so a config committed next to the project
  works from anywhere. A path that does not exist produces a warning at startup, not an error:
  the directory may be created by an earlier command in the chain.
- `timeout: 30s` — stop the command if it runs longer than this. Accepts any Go duration
  (`500ms`, `30s`, `1m30s`) — a bare number is rejected, because `timeout: 30` reads as seconds
  but means nothing without a unit. The command is stopped the same way Ctrl+C stops it — signal
  first, kill only if it does not exit — so whatever it printed before being stopped is still
  shown. Overrides `-timeout`; without either there is no limit.
- `restart: never | on-failure | always` — restart the command after it exits. `never` (the
  default) runs it once. `on-failure` restarts after any failure, including a command stopped by
  its own `timeout`. `always` restarts after a successful exit too — that is the whole difference
  between the two.
- `restartAttempts: 5` — how many times the command may be started in total. `0` or unset means
  no limit. Once the attempts run out the chain fails, and the exit code of the last failure is
  still passed through.
- `restartDelay: 1s` — how long to wait before the first restart; it doubles after each one, up
  to 30 seconds. The growing delay is what keeps `always` on an instantly-failing command from
  spinning the CPU.
- `disable: true` — disable a command without removing it from config. Disabled commands are shown in the flow preview
  and are skipped during execution. Default: `false`.
- `env: { KEY: value }` — environment variables for this command. They are **added to** the environment `parallel`
  itself runs with, not a replacement for it, so there is no need to restate `PATH`. On a key collision the value from
  the config wins. For `docker` commands the variables are passed to the **container** — see
  [Docker mode](#docker-mode).
- `envFile: .env` — load environment variables from a file, or several:
  `envFile: [ .env, .env.local ]`. Paths are resolved against the configuration file, like `dir`.
  A missing file is an error rather than silence — a skipped `envFile` would start the run with
  half the settings gone. The same key also works at the top level, next to `commands`, where it
  applies to every command.
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

### Environment variables

Four sources, from weakest to strongest:

| Source | Notes |
|---|---|
| the environment `parallel` itself runs with | inherited, never restated |
| top-level `envFile` | applies to every command |
| the command's own `envFile` | layered on top |
| the command's `env` | wins on a collision |

Values can reference other variables with `${VAR}` or `${VAR:-default}`. Substitution happens in
`env` values, in `dir` and in the elements of `cmd`:

```yaml
envFile: .env
commands:
  api:
    serve:
      dir: '${PROJECT_ROOT}/api'
      cmd: [ 'go', 'run', '${TARGET:-./cmd/api}' ]
      env:
        URL: 'http://localhost:${PORT}'
```

Three rules worth knowing:

- **`run:` is left alone on purpose.** Its body goes to the shell, which expands `$VAR` itself;
  a second expansion would either double up or disagree with what you expect from a shell. Write
  `run: 'echo $PORT'` and the shell resolves it.
- **A bare `$VAR` is never expanded** — only the `${...}` form. Command arguments contain lone
  dollars of their own (`awk '{print $1}'`), and eating them silently is worse than asking for
  braces. To write a literal `${`, use `$${`.
- **An undefined variable with no default is an error** naming the variable, not an empty string:
  an empty string quietly breaks paths and addresses, and you find out from the behaviour of the
  command rather than from the message.

A `.env` file is read as: full-line `#` comments and blank lines are skipped, an optional
`export ` prefix is dropped, the line is split at the **first** `=`, and a value wrapped in
matching quotes is unquoted as-is (no escape processing). In an unquoted value everything from
` #` onwards is a comment — quote the value if it must contain one.

> In Docker mode `env` becomes `-e KEY=VALUE`, so a top-level `envFile` reaches your containers
> too. That is usually what you want, but it does mean every variable in the file is passed.

### Docker mode

When `docker` section is used, the tool builds the final `docker` command for you, adds `--rm` by default (unless
`removeAfterAll: false`), applies `pull` policy and ports, and always runs with `pipe: true` for live logs. Because of
this, Docker commands start concurrently and the chain waits for them to finish.

`env` is turned into `-e KEY=VALUE` arguments, so the variables reach the **container** rather
than the `docker` client process. Beyond the image the mode also understands `ports`, `volumes`,
`network` and `args`:

```yaml
commands:
  services:
    db:
      docker:
        image: { name: postgres, tag: '17' }
        ports: [ '5432:5432' ]
        volumes: [ './data:/var/lib/postgresql/data' ]
        network: app-net
        args: [ '-c', 'max_connections=200' ]   # the container's own command
      env:
        POSTGRES_PASSWORD: secret
```

builds `docker run --name db --rm -p 5432:5432 -v <config-dir>/data:/var/lib/postgresql/data --network app-net -e POSTGRES_PASSWORD=secret postgres:17 -c max_connections=200`.

> **`cmd` is the `docker` subcommand, not the container's command.** `cmd: 'exec'` selects
> `docker exec`, which reads exactly backwards from what you might expect. Whatever should run
> *inside* the container belongs in `args`. The name cannot be fixed — `docker.*` is part of the
> frozen `v1` contract.

Flags always come before the image name and `args` always after it, because `docker` treats
everything following the image as the container's command. In a volume, a host path starting
with `./` or `../` is resolved against the configuration file, the same as `dir`; anything else
is left alone, so `data:/var/lib` stays a named volume instead of turning into a directory.

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
- A whole shell line is written in `cmd` (with a pipe or `&&`) and the command is not found
    - `cmd` is a program plus its arguments, not a shell line. Invoke the shell explicitly:
      `cmd: [ 'sh', '-c', 'a && b' ]`
- Configuration is not found
    - The error lists the names that were searched. Note that `-f` is taken literally and is
      never searched for upwards: a typo in `-f` must fail rather than silently pick up someone
      else's config from a parent directory
- Docker command keeps running after Ctrl+C
    - The tool sends signal to the process group; ensure your containerized process handles `SIGTERM` and stops promptly
- YAML error: “invalid flow configuration”
    - The tool validates that each chain has at least one command and each command has a non‑empty `cmd`

## Compatibility

Starting with `v1.0.0` the following is frozen and will not change without a `v2`:

- **CLI flags** — `-f <path>`, `-v`, `--version`, `-list`, `-dry-run`, `-except`, `-no-color`,
  `-keep-going`, `-timeout`;
  positional arguments select chains and `--` starts config-less mode; the default config name
  `.parallelrc.yaml`
  (`.parallelrc.yml` is also accepted), looked up in the current directory and its parents.
- **Exit codes** — `0` on success; `1` on a startup or configuration error; `124` on a timeout;
  a failing command's own exit status is passed through.
- **Configuration schema** — the top-level keys `commands`, `failFast` and `envFile`, and the command
  fields `cmd`, `run`, `timeout`, `restart`, `restartAttempts`, `restartDelay`, `envFile`, `dir`,
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

Русская версия документации — [README.ru.md](README.ru.md).
