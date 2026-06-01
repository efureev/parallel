# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
