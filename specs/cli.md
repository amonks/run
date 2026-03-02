# CLI

## Overview

The `run` CLI (root package) is the main entry point. It parses flags, loads tasks, selects a UI, and orchestrates execution. It can be installed with `go install github.com/amonks/run@latest`.

## Usage

```
run [flags] <task>
```

Takes one argument: the task ID to run. Looks for `tasks.toml` in the current directory (or the directory specified by `-dir`).

## Flags

- `-ui=string`: force `tui` or `printer` UI (auto-detected by default).
- `-dir=string` (default `"."`): look for a root taskfile in this directory.
- `-list`: display the task list and exit. With a task ID, shows that task's dependencies.
- `-skip=task-id`: skip a task, replacing it with a no-op stub. Can be passed more than once. Short stubs print "skipping" and exit immediately; long stubs become ready immediately and block until interrupted.
- `-version`: display version info and exit.
- `-help`: display help text and exit.
- `-credits`: display open source credits and exit.
- `-contributors`: display contributors list and exit.
- `-license`: display license info and exit.

## UI Selection

When `-ui` is not specified:

1. If stdout is not a TTY: use `Printer`.
2. If the root task type is `"long"`: use `TUI`.
3. Otherwise: use `Printer`.

## Execution Flow

1. Parse flags. Handle info flags (`-version`, `-help`, etc.) and exit.
2. Load tasks via `taskfile.Load(dir)`.
3. If `-skip` flags were given, validate that each task ID exists, then replace those tasks with `SkipTask` stubs and rebuild the library.
4. If no task ID and `-list`: print task list and exit.
5. If no task ID: print help and exit.
6. Validate task ID exists.
7. If `-list` with task ID: print that task's dependency subtree and exit.
8. Determine UI mode based on flags and root task type.
9. Set up signal handling via `signal.NotifyContext` (SIGHUP, SIGTERM, SIGINT, SIGQUIT).
10. If TUI mode: call `tui.Start(ctx, stdin, stdout, dir, allTasks, taskID)` which creates the runner internally with `RunTypeLong` and blocks.
11. If printer mode: create `printer.New(gutterWidth, stdout)`, create `runner.New(RunTypeShort, ...)`, and call `r.Start(ctx)` which blocks.

## Exit Behavior

- Context cancellation (via signal) triggers exit code 0 with "Canceled" message.
- Errors trigger exit code 1.
- In TUI mode, the runner uses `RunTypeLong` (keepalive), so it stays alive until the user quits the TUI. This means `-ui=tui` with a short task shows the task completing and keeps the TUI open for output inspection.

## Task List Output

`-list` renders each task with:
- Color-coded ID (via `color.RenderHash`).
- Type (italic).
- Description (dedented and indented).
- Dependencies, triggers, and watches as bulleted lists.

## Version Info

Displays version, revision, and release date from build metadata. Dirty builds are labeled.
