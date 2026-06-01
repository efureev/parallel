# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
