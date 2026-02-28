# CLI

## Overview

The `run` CLI (`cmd/run`) is the main entry point. It parses flags, loads tasks, selects a UI, and orchestrates execution.

## Usage

```
run [flags] <task>
```

Takes one argument: the task ID to run. Looks for `tasks.toml` in the current directory (or the directory specified by `-dir`).

## Flags

- `-ui=string`: force `tui` or `printer` UI (auto-detected by default).
- `-dir=string` (default `"."`): look for a root taskfile in this directory.
- `-list`: display the task list and exit. With a task ID, shows that task's dependencies.
- `-version`: display version info and exit.
- `-help`: display help text and exit.
- `-credits`: display open source credits and exit.
- `-contributors`: display contributors list and exit.
- `-license`: display license info and exit.

## UI Selection

When `-ui` is not specified:

1. If stdout is not a TTY: use `Printer`.
2. If the run type is `RunTypeShort`: use `Printer`.
3. Otherwise: use `TUI`.

## Execution Flow

1. Parse flags. Handle info flags (`-version`, `-help`, etc.) and exit.
2. Load tasks via `run.Load(dir)`.
3. If no task ID and `-list`: print task list and exit.
4. If no task ID: print help and exit.
5. Create `Run` via `run.RunTask(dir, tasks, taskID)`.
6. If `-list` with task ID: print that task's dependency tree and exit.
7. Select and instantiate UI.
8. Start UI in a goroutine; wait for readiness signal.
9. Start Run in a goroutine.
10. Wait for either completion or a signal (SIGHUP, SIGTERM, SIGINT, SIGQUIT).

## Exit Behavior

- The first component to exit (UI or Run) determines the exit reason.
- If the UI exits before a short run completes, that is an error.
- If `-ui=tui` was explicitly set, the TUI stays open after the run completes so the user can inspect output.
- Context cancellation triggers exit code 0 with "Canceled" message.
- Errors trigger exit code 1.

## Task List Output

`-list` renders each task with:
- Color-coded ID (via `color.RenderHash`).
- Type (italic).
- Description (dedented and indented).
- Dependencies, triggers, and watches as bulleted lists.

## Version Info

Displays version, revision, and release date from build metadata. Dirty builds are labeled.
