# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.0] - unreleased

### Added

- **`-keep-going` and the `failFast` configuration key.** The failure of one chain used to stop
  every other one, with no way to turn that off. For a dev stack it is the right default — once
  the API is gone there is little point in keeping the frontend up — but in CI it is actively
  harmful: running the linter, the tests and the build as three chains, you want every failure at
  once rather than the first one and then two more runs. An explicit flag beats the file, so
  `-keep-going=false` forces the default back for a single run.

  The switch only affects chains relative to each other. Inside a chain nothing changes: a failed
  non-`pipe` command still skips the rest of *its* chain, because commands there are usually
  dependent and continuing past a failed `migrate` would work against a broken state.

  One consequence worth knowing: with `-keep-going` a failing chain no longer cuts short a
  long-running sibling, so a run that includes a dev server will not finish on its own.
- **`CONTRIBUTING.md`.** The requirements here are stricter than usual and none of them are
  visible from the code: the linter must be run through `make lint` rather than from `PATH`,
  `GOOS=windows go vet` catches breakage the local build never will, layer boundaries are
  enforced by `depguard`, and four invariants in the process runner fail silently when broken.
  Until now a contributor learned these from red CI, one per push.
- **Pre-release tags are published as pre-releases.** A tag with a SemVer pre-release suffix
  (`v1.2.0-rc1`, `v2.0.0-beta.1`) used to be published as an ordinary release, which made GitHub
  mark it "latest". The build still runs and still uploads binaries — the artifacts are the whole
  point of a release candidate — but the release itself is now flagged accordingly.
- **Dependencies between chains: `needs`, `ready` and `maxParallel`.** Chains used to be entirely
  independent, so "bring up the database, wait until it accepts connections, then start the API"
  could not be expressed at all — the single biggest reason to reach for `docker compose` or
  `overmind` instead.

  A command declares when it counts as ready with exactly one of `tcp`, `exec` or `logLine`, and
  a chain waits for others with `needs`. A chain that declares no readiness condition is ready
  once it has finished successfully — migrations and builds do not listen on a port, and waiting
  for them means waiting for the exit. A chain that does declare one opens the way as soon as
  the condition holds, without having to finish, which is the entire point for a server that
  never does.

  `needs` is a reserved key inside a chain, since every other key there is a command name; using
  it as a command name now fails with a message that says so. Cycles are refused before anything
  starts and the error spells the cycle out. Selecting a subset pulls in what it depends on, so
  `parallel api` runs `db` too.

  `maxParallel`, or the `-jobs` flag that overrides it, caps how many chains run at once. Waiting
  for a dependency happens before a slot is taken rather than after, so a limit cannot deadlock a
  graph — the obvious implementation with `errgroup.SetLimit` would have let a dependent hold the
  only slot while waiting for the predecessor that needs it.

  Chains that never start because a dependency failed are reported as `skipped` rather than
  `failed`: "never began" and "was cut short" are different things, and the summary is read
  precisely when telling them apart matters.
- **Docker mode understands `volumes`, `network` and `args`.** Until now it covered only the
  image, ports, `pull` and `--rm`, so any non-trivial container sent you back to writing a raw
  `cmd:` — the mode was a showcase rather than a shortcut. Relative host paths in a volume
  (`./data:/var/lib`) are resolved against the configuration file, like `dir`; named volumes and
  absolute paths are left alone, so `data:/var/lib` does not silently become a directory.

  `args` is the container's own command and is placed strictly after the image name, because
  that is how `docker` reads it. The existing `cmd` field is the `docker` **subcommand**
  (`run`, `exec`) despite reading like the opposite; it could not be renamed, since `docker.*`
  is part of the frozen `v1` contract, so the container command got a new field instead and the
  trap is now spelled out in the README.

  Docker commands also stop repeating their whole command line in front of every output line:
  the arguments are assembled by the tool and, with volumes and a container command, grew longer
  than the output itself. They now show just the command name, as the `run:` form already did.
  An explicit `format.cmdName` still wins.
- **`envFile` and `${VAR}` substitution.** Environment variables already live in a project's
  `.env`, and nobody is going to restate them in YAML — until now the only way to set
  `DATABASE_URL` was to copy it by hand next to the file that already had it. `envFile` accepts
  one path or a list, works both at the top level (for every command) and on a single command,
  and resolves paths against the configuration file the way `dir` does. A missing file is an
  error rather than silence: a skipped `envFile` would start the run with half the settings gone.

  Precedence runs from the process environment through the top-level files and the command's own
  files to `env`, which wins. Values can reference variables with `${VAR}` or `${VAR:-default}`,
  expanded in `env` values, in `dir` and in the elements of `cmd`.

  **The body of `run:` is deliberately left alone** — it goes to the shell, which expands `$VAR`
  itself, and a second expansion would either double up or disagree with what a shell is expected
  to do. For the same reason a bare `$VAR` is never expanded anywhere: command arguments contain
  lone dollars of their own, as in `awk '{print $1}'`. Only the `${...}` form is substituted, and
  `$${` writes a literal one. An undefined variable with no default is an error naming the
  variable rather than an empty string, because an empty string quietly breaks paths and
  addresses.
- **A restart policy per command: `restart`, `restartAttempts`, `restartDelay`.** A dev server
  that died on a compile error used to take the whole run down with it — the chain broke and, by
  default, its siblings were cancelled too, so a typo cost a full manual restart. This is the one
  reason people still keep `nodemon` next to this tool.

  `on-failure` restarts after any failure, including a command stopped by its own `timeout`;
  `always` restarts after a successful exit as well, which is the whole difference between them.
  Attempts are unlimited unless `restartAttempts` says otherwise, because the main case is a dev
  server that should come back up for as long as you keep fixing it. What protects against a
  busy loop is not a counter but the delay, which doubles after every restart up to 30 seconds.

  A command being restarted has not failed yet, so sibling chains keep running while the attempts
  last. Cancellation is checked before the policy: after Ctrl+C nothing is restarted, otherwise a
  command would come back up faster than it could be stopped. When the attempts run out the chain
  fails and the exit code of the last failure is still passed through — the "gave up after N
  attempts" note wraps the original error rather than replacing it.
- **A `timeout` per command and a global `-timeout` flag.** A hung command used to hold the whole
  run until Ctrl+C, which in CI is the worst outcome: instead of a report you get a job killed by
  the overall limit, with nothing in the log to say which command stalled. `timeout: 30s` on a
  command overrides the flag; without either there is no limit. A bare number is rejected — the
  error says a unit is required, because `timeout: 30` reads as seconds but means nothing on its
  own.

  The command is stopped through the same ladder as Ctrl+C — signal to the process group, kill
  only if it does not exit, then a bounded wait for the output to drain. Killing it outright
  would have been simpler but would lose the tail of the output, and that tail is exactly what
  explains where the command stalled.

  A timed-out run exits with `124`, following the convention of `timeout(1)`: a stopped process
  has no exit status of its own, so one has to be chosen. In the summary such a chain gets its
  own `timed out` status rather than a plain `failed`.
- **Better field suggestions for typos in long names.** `restartAttemps` used to be answered with
  "possibly meant restart", because a shared prefix outranked a closer match. Edit distance is
  now tried first and the prefix rule only steps in when nothing is close — it exists for endings
  such as `pipeline` against `pipe`, which are four edits apart.
- **A warning for top-level keys that look like a typo.** `failFats: false` used to do nothing
  and say nothing. Unknown top-level keys stay accepted — rejecting them would break
  configurations that keep YAML anchors there, and the schema has been frozen since `v1.0.0` — so
  only keys close to a known name are reported, and `x-common` or `_defaults` remain silent.

### Fixed

- **The exit code was not reproducible when several chains failed at once.** In fail-fast mode a
  sibling's failure is masked by the cancellation that its neighbour triggered, so the same
  configuration returned different codes from run to run. Under `-keep-going` no failure is
  masked, and the documented rule — the code of the first failure in configuration order — now
  holds on every run.

## [1.1.0] - 2026-08-13

### Added

- **String command form `run:`.** `run: 'npm run dev'` is executed through the shell (`sh -c`,
  `%COMSPEC% /c` on Windows), so `&&`, pipes, globs and variable expansion all work in it. The
  `cmd:` form is unchanged and still taken literally; specifying both at once is an error rather
  than a silent preference for one of them. In the output prefix a shell-form command shows its
  name only: its single argument is the entire command line, which would turn every log line
  into noise.
- **Running with no configuration file.** `parallel -- 'go run ./cmd/api' 'yarn dev'` builds the
  Flow straight from the arguments: one chain per command, all of them streaming. The chain name
  is taken from the first word of the command, and identical names are told apart by a number.
- **Chain selection from the command line.** `parallel api ui` runs only the named chains,
  `-except worker` runs everything but the listed ones. Until now the only way was
  `disable: true` in a file that lives in the repository. An unknown name is an error that lists
  the available ones: otherwise a typo would produce a "successful" run in which nothing
  executed. A chain keeps its color when a subset is selected.
- **`-list` and `-dry-run`.** The first shows what the configuration defines, the second shows
  what exactly would run; both exit without starting anything.
- **Run summary.** When more than one chain is involved, a chain / status / duration table is
  printed at the end, with the reason next to a failed one. The `stopped` status separates a
  chain that was cut short — by a sibling failure or by a signal — from one that finished on its
  own.
- **`NO_COLOR`, `FORCE_COLOR` and the `-no-color` flag.** The no-color.org convention used to be
  ignored: coloring was decided solely by whether the output was a terminal.
- **The configuration file is looked up in parent directories.** Lookup used to happen in the
  current directory only, so running from a subdirectory of a project failed with "config file
  not found" — even though the project does contain the file. The search now walks up to the
  filesystem root, the way `git` finds its own configuration, and the nearest directory wins.
  Relative `dir` values are resolved against the file that was found, so a configuration at the
  project root works from any subdirectory.
- **`.parallelrc.yml` is accepted.** Both spellings are equally common, and a user who named the
  file `.yml` used to get an error quoting a name they had never written. Within one directory
  `.yaml` is preferred over `.yml`.
- The "configuration not found" message lists the names that were searched and states that
  parent directories were included.

### Fixed

- **A chain cut short by fail-fast looked successful.** Context cancellation is not treated as an
  error, so such a chain ended up in the summary as having finished normally. It now carries a
  status of its own.

### Changed

- **The path given with `-f` is taken literally and is never searched for upwards.** A typo in an
  explicit path must fail rather than quietly pick up someone else's configuration from a parent
  directory. `-f ""` remains an error.

## [1.0.0] - 2026-08-13

The first major release. A complete rework of the internals with no change to command-line
behaviour: configurations that worked on `0.x` keep working unmodified, and the output for the
same configuration matches the previous one character for character.

**Breaking changes**

- **The import path `github.com/efureev/parallel/src` no longer exists.** All code moved to
  `cmd/parallel` and `internal/*`, which means the project no longer exposes an importable Go API
  at all. `parallel` is a command, not a library. Installation via `go install` is unchanged.
- **Shutdown on Windows became graceful.** The child process group receives a `CTRL_BREAK`
  console event (handled by the Node.js, Python, .NET and Go runtimes), and only then, on
  timeout, is it killed. Previously the process was killed outright, with no chance to close
  connections.
- **A second Ctrl+C is no longer ignored.** The first one stops everything politely, the second
  kills every process immediately, the third exits with status `130`.

**The frozen `v1` contract** covers flags, exit codes, the configuration schema and execution
semantics; the details live in the Compatibility section of `README.md`. The log line format, the
palette and error wording are deliberately outside it and may change. The module path stays
without a `/vN` suffix.

### Fixed

- **A typo in a configuration field name was silently ignored.** `pipeline` instead of `pipe`,
  `diir` instead of `dir` — the file was accepted, the command ran differently from what its
  author intended, and the only clues were indirect. An unknown field is now an error that points
  at the position and suggests the closest known name.
- **`parallel -h` exited with status 1** and printed "Failed to parse flags", so asking for help
  counted as a failure and broke scripts. The help text was rewritten along the way: the
  generated one listed `-v` and `-version` as two separate entries and gave no examples.
- **`env` in `docker` mode never reached the container.** The variables were applied to the
  `docker` client process, and the client does not pass its own environment to the container, so
  anything written next to a `docker` section went nowhere. They are now turned into
  `-e KEY=VALUE` arguments. Regular commands are unaffected: for them `env` is still the process
  environment.
- **The exit code did not distinguish causes of failure.** Every error collapsed into `1`, so a
  script could not tell "the command exited with 2" from "the configuration is unreadable". The
  exit code of the command whose failure stopped the run is now propagated; configuration and
  startup errors still give `1`.
- **A relative `dir` was resolved against the process working directory** rather than against the
  configuration file. A configuration only worked when started from the "right" place, even
  though it sits next to the project. Absolute paths are unaffected, and for a configuration at
  the project root started from that same root nothing changes.
- **The failure of the second and later piped commands in a chain was lost:** waiting on the
  group returned only the first error. Errors are now collected by command position, so both
  their set and their order no longer depend on the scheduler.
- **A missing working directory was reported as a problem with the executable** —
  "fork/exec /bin/pwd: no such file or directory" — when it was the directory that was absent.
  The message now says exactly that: `working directory "…" does not exist`.
- **Every piped command lost the last lines of its output.** `cmd.Wait()` closes the pipes as
  soon as the process exits, and output reading was cancelled before the readers had drained the
  remainder. Roughly 60 lines per command were lost regardless of the total volume — that is,
  precisely the result of the work: the `go test` verdict, the error message before exit, the
  tail of a build. The shutdown order was inverted: the output is drained to EOF first, and only
  then is the process status collected. Guarded by `TestE2EOutputIntegrity`, which requires that
  every single line reaches the user.
- **Output lines from different chains could overlap.** Writes to the shared descriptor from many
  goroutines were not synchronized, which produced interleaving on lines longer than `PIPE_BUF`.
  Writes are now serialized.
- **A goroutine leak in `ExecuteParallel`.** The cancellation watcher waited on the parent context
  and, if that was never cancelled, lived until the process ended.
- **Chain errors were lost.** The first one to arrive was returned and the rest discarded, and
  which one arrived first depended on the scheduler. All of them are now returned via
  `errors.Join`.
- **A configuration parse error did not point at the position.** The message now contains the
  file, line, column and a source fragment with a marker at the offending spot.
- **`FlowBuilder.Build` returned an empty `Flow` instead of an error** when the `commands` key was
  missing.

### Added

- **`env:` in the command schema** — environment variables for a specific command. They extend
  the process environment rather than replacing it; on a key collision the value from the
  configuration wins.
- **The `-log-level` flag** (`debug`, `info`, `warn`, `error`). Debug messages existed in the code
  but never reached the user and there was no way to turn them on — which is exactly what is
  needed when investigating a stuck process.
- **An `examples/` directory** with two self-contained configurations that work out of the box.
- **A warning about a missing working directory** at startup. A warning rather than an error: the
  directory may be created by an earlier command in the chain.
- Benchmarks for the hot path and an end-to-end output integrity check.

### Changed

- **The project layout follows the Go standard**: `cmd/parallel` plus `internal/*` split by layer
  (`flow`, `config`, `runner`, `ui`, `cli`, `buildinfo`). The direction of dependencies between
  layers is enforced by the `depguard` linter rather than by convention.
- **Logging is isolated behind an interface of its own**: the library is named in exactly one file
  of the project, so a change of its major version no longer touches the rest of the code.
- **Dependencies:** `reggol` updated from `0.4.1` to `1.2.1`, YAML parsing moved from
  `gopkg.in/yaml.v3` (no releases since 2022) to `goccy/go-yaml`, and `golang.org/x/sync` and
  `golang.org/x/sys` were added. The minimum Go version is now 1.26.
- **Output performance.** Formatting of block output was quadratic: 10,000 lines took 4.59 GB of
  allocations and a third of a second — now it is 926 KB and 0.13 ms. The end-to-end cost of a
  line dropped from 900 to 272 ns, and throughput on a real pipe grew from 268,000 to 803,000
  lines per second. Output is buffered and flushed on a timer, so interactivity is preserved.
- **Shutdown timings are configurable** rather than hard-coded constants. The full test run got
  faster, from 6.6 to 4.2 seconds, and the slowest test from 3.3 to 0.3 seconds.
- `build.sh` names the file after the real `GOARCH`: an arm64 build no longer produces a file with
  an `.x64` suffix.

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
[1.1.0]: https://github.com/efureev/parallel/releases/tag/v1.1.0
[1.2.0]: https://github.com/efureev/parallel/releases/tag/v1.2.0
