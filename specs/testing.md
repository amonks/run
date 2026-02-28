# Testing

## Overview

Run's tests are organized into unit tests, example tests, and snapshot-based integration tests. Tests exercise real behavior using the actual task engine and printer UI.

## Test Files

- `pkg/run/*_test.go`: unit and integration tests for the core package.
- `pkg/logview/*_test.go`: tests for the log viewer.
- `internal/color/hash_test.go`: tests for color hashing.
- `cmd/run/first_test.go`: tests for the synchronization helper.

## Example Tests

Three example tests demonstrate public API usage:

- `example_test.go`: basic Run + TUI usage.
- `example_bring_your_own_tasks_test.go`: creating custom `FuncTask` implementations.
- `example_bring_your_own_ui_test.go`: implementing a custom `UI`.

## Snapshot Integration Tests

Located in `pkg/run/testdata/snapshots/`. Each snapshot is a directory containing:

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

## Self-Hosting

Run uses itself for its own development tasks (defined in `tasks.toml`). The `validate` task runs: `generate`, `test`, `vulncheck`, `staticcheck`, `snapshot-cli`, and `shellcheck`.
