# Testing

## Overview

Run's tests are organized into unit tests, example tests, and snapshot-based integration tests. Tests exercise real behavior using the actual task engine and printer UI.

## Test Files

- `task/*_test.go`, `taskfile/*_test.go`, `runner/*_test.go`: unit and integration tests for the core packages.
- `runner/runner_test.go`: event loop tests exercising the runner's message dispatch, dependency resolution, triggers, file watching, dynamic Add/Remove, and Invalidate.
- `logview/*_test.go`: tests for the log viewer.
- `internal/color/hash_test.go`: tests for color hashing.
- `internal/seq/seq_test.go`: tests for the sequence assertion helper.
- `first_test.go`: tests for the synchronization helper.

## Example Tests

Three example tests demonstrate public API usage:

- `example_test.go`: basic Run + TUI usage.
- `example_bring_your_own_tasks_test.go`: creating custom `FuncTask` implementations.
- `example_bring_your_own_ui_test.go`: implementing a custom `UI`.

## Snapshot Integration Tests

Located in `runner/testdata/snapshots/`. Each snapshot is a directory containing:

- `tasks.toml`: task definitions for the test scenario.
- `out.log`: expected printer output.
- `fail.log`: written on test failure for comparison (not checked in).

### Test Scenarios

Scenarios cover: short task success/failure, long task behavior, dependencies, triggers, environment variables, JSON output, monorepo nesting (three layers), and combined short/long groups.

### How Snapshots Work

1. Load tasks from the snapshot directory.
2. Run the task named `test` using the Printer UI.
3. For short runs, wait for completion. For long runs, wait 5 seconds then cancel.
4. One second into the test, delete a `changed-file` to exercise file watching.
5. Compare output against `out.log`. Output is deinterleaved (grouped by task ID) before comparison to handle non-deterministic interleaving.
6. On mismatch, write `fail.log` and show a diff.

### Managing Snapshots

- `run reset-snapshot-failures`: removes all `fail.log` files.
- `run overwrite-snapshots`: replaces `out.log` with `fail.log` for each failing snapshot.

## CLI Snapshots

The `snapshot-cli` task validates CLI output for `-version`, `-help`, `-credits`, `-contributors`, `-license`, and `-list` flags against checked-in snapshots in `testdata/cli_snapshots/`.

## Test Fixtures

Located in `internal/fixtures/`:

- **`task.go`**: Mock `run.Task` implementation with builder API (`WithWatch`, `WithDependencies`, `WithTriggers`, `WithOutput`, `WithReady`, `WithExit`, `WithCancel`, `WithImmediateFailure`). Supports three execution modes: immediate return, blocking until exit signal, and blocking until context cancellation.
- **`writer.go`**: Recording `MultiWriter` that captures per-stream and combined output with `[id]` prefixes. Strips ANSI escape codes for assertion-friendly output.

Located in `internal/seq/`:

- **`seq.go`**: `ContainsSequence` checks that a list of lines contains a given subsequence in order, and that no subsequence element appears outside its matched position. `AssertStringContainsSequence` is a test helper that splits on newlines first.

## Event Loop Tests

`runner/runner_test.go` exercises the runner's single-channel event loop (the `handleMessage` dispatch in `runner.go`) through 15 black-box tests:

1. Short run, no deps, succeeds
2. Short run, no deps, fails (error propagation)
3. Short run with dependency chain (ordering)
4. Failing dependency prevents dependent
5. Context cancellation (clean shutdown)
6. Watch-triggered restart (long task)
7. Trigger completion causes main task restart
8. Watch event on trigger causes cascade restart
9. Dependency rerun does not restart long task
10. JSON output is prettified
11. Long task onReady enables dependent
12. Dynamic Add
13. Dynamic Remove
14. Invalidate restarts running task
15. Watch-triggered restart (short dep in long run)

Tests use `watcher.Mock()` for synthetic file system events and `sync/atomic` counters for race-free start counting. All tests pass under `-race`.

## Self-Hosting

Run uses itself for its own development tasks (defined in `tasks.toml`). The `validate` task runs: `generate`, `test`, `vulncheck`, `staticcheck`, `snapshot-cli`, and `shellcheck`.
