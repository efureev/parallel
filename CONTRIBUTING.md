# Contributing

Thanks for wanting to help. This project has a few requirements that are stricter than usual,
and none of them are obvious from the code alone — this page exists so you learn them here
rather than from a red CI run, one per push.

Everything below is enforced by the pipeline. If you follow this page, CI passes.

## Getting set up

You need Go 1.26 (the version in `go.mod`) and Docker. Docker is not optional: it is how the
linter is run, see below.

```shell
git clone https://github.com/efureev/parallel && cd parallel
make test          # linter + tests, exactly as CI runs them
```

`make` targets run inside containers so a clean machine needs nothing but Docker:
`make fmt`, `make lint`, `make gotest`, `make test`, `make clean`.

## Run the linter through `make lint`, never from your `PATH`

This is the single most important rule here.

The pipeline pins `golangci-lint` **v2.12** in both `.github/workflows/test.yml` and
`docker-compose.yml`. The binary in your `PATH` is almost certainly a different version, and the
difference is not cosmetic: a 2.7 binary silently missed nine findings that v2.12 reports —
`goconst`, `dogsled`, `funlen`, `misspell`, `mnd`, `nilnil`, `gosec`, `gocognit`, `lll`. Every
one of them would have failed CI after a green local run.

```shell
make lint          # v2.12 in a container — the only linter check that means anything
```

Tests are the opposite: `go test ./...` locally is faster than the container and gives the same
result. Use `make gotest` when you want to be certain, `go test` while you work.

## Before you commit

```shell
go build ./... && go vet ./...
GOOS=windows go vet ./...      # see "Windows" below — this one catches real breakage
gofmt -l .                     # must print nothing
make lint
go test ./... -race -shuffle=on
```

### Windows is a supported platform, and test files are compiled for it too

CI runs the test suite on Linux, macOS **and** Windows. `GOOS=windows go vet ./...` is not
paranoia: a helper defined in a `//go:build !windows` file but used from a shared `_test.go`
compiles fine on your machine and breaks the Windows runner every time.

- Platform-specific code lives in build-tagged files (`process_group_unix.go` /
  `process_group_windows.go`).
- Never reference `syscall.SysProcAttr`, `Setpgid` or signal constants from an untagged test.
- Never hard-code `sleep`: Windows has no such utility. Use the `sleepCmdNameArgs` helper,
  which expands to `ping -n` there. New external commands follow the same pattern — a pair of
  tagged files with one shared signature.

## Layer boundaries are checked by the compiler chain, not by convention

Dependencies point one way only:

```
cli → {config, flow, runner, ui, buildinfo}
runner → {flow, ui}
config → flow
ui → flow
flow, buildinfo → nothing
```

In practice: `flow` is the domain and imports no other internal package; `config` knows nothing
about `runner`, `ui` or `cli`; `ui` knows nothing about processes; `runner` knows nothing about
YAML. This is enforced by `depguard` in `.golangci.yml`, so breaking it is a build failure, not
a review comment. A violation means the change belongs in a different package — do not relax the
rule to fit the code.

## Four invariants in `internal/runner`

Breaking any of these produces no compile error and no failing test in your editor. Each was
broken once already.

1. **Drain output to EOF first, call `cmd.Wait()` second.** `Wait` closes the pipes as soon as
   the process exits, so the reverse order silently loses the tail of the output — it used to
   lose the last ~60 lines of *every* piped command, which is exactly the part that matters:
   the test verdict, the error before exit. `TestE2EOutputIntegrity` guards this.
2. **Cancelling a command is not cancelling its output.** Reading ends by itself at EOF. Forcing
   it to stop is the emergency path only, used when a process was killed and EOF never arrived.
3. **`supervise` is the only caller of `waitFn` and the only closer of `waitDone`.** A second
   `Wait` gives you a race and an undefined exit status.
4. **Output is read directly, with no goroutine in between.** A channel per line was removed
   deliberately; responsiveness comes from closing the descriptor, not from a middleman.

## Adding a field to the YAML schema

A new field has to be threaded through five places. Missing any of them compiles cleanly and the
field is silently ignored:

1. the `command` struct in `internal/config/loader.go` (yaml tag) **and** `knownCommandFields`,
   which powers the "did you mean" hint;
2. the mapping in `internal/config/builder.go` — both `createRegularCommand` **and**
   `createDockerCommand`;
3. the field on the domain type in `internal/flow/command.go`;
4. the preview in `internal/ui/flow_reader.go`, or the field is invisible in `-dry-run`;
5. the Fields section of **both** `README.md` and `README.ru.md`.

Plus a commented example in `examples/full.yaml`. Field names are camelCase (`removeAfterAll`,
`failFast`, `restartAttempts`) — match the existing schema, not the prose in any planning doc.

## Tests

- Tests live in the same package as the code and may touch unexported functions: that is where
  the invariants worth guarding are.
- Anything that forks a real process starts with `requireIntegration(t)` and is skipped under
  `-short`. Fast loop: `go test ./... -short`.
- Always run with `-race -shuffle=on`. This code is concurrent throughout; a run without `-race`
  proves nothing.
- **Do not synchronise with `time.Sleep`.** Use channels and barriers. A chain failure cancels
  its siblings, so a test that assumes both got far enough is a flaky test, not a slow one.
- Timeouts are injected (`WithTimeouts`), not hard-coded — tests run on milliseconds. No single
  test should take longer than half a second.
- **Coverage has a floor per package**, listed in `.coverage-thresholds` and checked by
  `make cover` (and by CI, on pull requests too). The limits differ by layer on purpose: the
  domain and the config parser are held near-total, `cli` is not, because it is wiring that is
  exercised through `runner`. Dropping a limit is a deliberate edit in the same commit, with the
  reason in the commit message — not a silent drift.

## Style

- **Comments are in Russian; `CHANGELOG.md` and the primary `README.md` are in English.** Keep
  writing in the language of the file you are editing.
- Comments explain *why*, not *what*. If a line looks like a mistake and is not, say what would
  break if someone "fixed" it.
- The linter enables more than forty checks with `default: none`, among them `wsl_v5` and
  `nlreturn` (mandatory blank lines, notably before `return`), `godot` (declaration comments end
  with a period), `lll` at 120 columns, `gochecknoglobals`, `mnd`, and `forbidigo`, which bans
  `fmt.Print*` — output goes through the logger.
- Disabling an enabled linter to make your change pass is not an option; fix the code. A targeted
  `//nolint` must name the linter and give a reason (`nolintlint` requires it).
- **The logging library must not leave `internal/ui`.** Every other layer talks to the
  `ui.Logger` interface and never imports `reggol` directly. This is not taste: that isolation
  once turned a major version bump into a change in a handful of files instead of a rewrite.
  Inside `ui` the adapter lives in `logger_reggol.go`; the palette files also reach for the
  library's colour constants, which is the one place the boundary is thinner than it looks.
  Nothing outside `internal/ui` may import it.

## Commits and pull requests

- Commit subjects use a type prefix: `feat:`, `fix:`, `ci:`, `docs:`, `refactor:`.
- Any user-visible change needs a `CHANGELOG.md` entry in the current unreleased section,
  in English, in the style of the surrounding entries: a bold claim first, then prose that
  explains what the old behaviour cost — not just what changed.
- Keep `README.md` and `README.ru.md` in step. They are section-for-section identical, and a
  change to one without the other is an unfinished change:
  ```shell
  paste <(grep '^#\{2,3\} ' README.md) <(grep '^#\{2,3\} ' README.ru.md)
  ```

## Compatibility

Since `v1.0.0` the CLI flags, exit codes, configuration schema and execution semantics are
frozen; the exact log format, colour palette and error wording deliberately are not. The
authoritative list is the **Compatibility** section of `README.md`.

Adding to the schema is fine. Changing what an existing field means is not — it costs a major
version, `/v2` module path included. If your change needs that, open an issue before writing
the code.
