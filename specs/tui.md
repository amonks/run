# Interactive TUI

## Overview

The interactive TUI provides a split-pane terminal interface for viewing task output, built on the BubbleTea framework. It is used when stdout is a TTY and any running task is long.

## Construction

```go
func NewTUI(run *Run) UI
```

Returns a `UI` that renders an interactive terminal interface.

## Layout

The TUI has four focus areas:

- **Menu** (left pane): task list with status indicators.
- **Logs** (right pane): log output for the selected task, powered by `logview.Model`.
- **Search**: regex search within the active log pane.
- **Help**: full-screen overlay showing key bindings.

### Task List

- Displays all task IDs plus `@interleaved` (combined output from all tasks).
- Selected task indicated with `>` marker.
- Status spinners: `Jump` spinner for short tasks, `Hamburger` spinner for long tasks.
- Status indicators: ` ` (not started), spinner (running/restarting), `✓` (done), `×` (failed).
- Internal tasks (prefixed with `@`) show a blank spinner.

### Interleaved View

The TUI creates a `Printer` UI internally and feeds its output to an `@interleaved` log view. This gives users a combined view of all task output alongside individual task views.

## Input Handling

### Mouse Support

- Click task names in the menu to select them.
- Mouse events within the log pane are forwarded to `logview`.
- Mouse cell motion is enabled.

### Keyboard

Key handling depends on the current focus area. The TUI supports vim-style navigation, search, and task management. Help overlay (toggled with `?`) shows all available bindings.

### Quit Behavior

- `Ctrl+C` always quits.
- `q` quits, but requires two presses within a short window (resets after timeout).

## Writers

Each task ID gets a `tuiWriter` that:

1. Writes to the interleaved printer (unless the write is from `@interleaved` itself).
2. Sends a `writeMsg` to the BubbleTea program for the task-specific log view.

## Program Configuration

- Alt screen mode (fullscreen).
- 120 FPS rendering.
- Mouse cell motion tracking.
- Zone-based click detection via `bubblezone`.

## Styling

Styles are computed dynamically based on terminal width, height, and current focus area. The TUI uses Solarized-derived colors from `internal/color`.

## UI Selection Logic

The TUI is selected automatically when:

1. Stdout is a TTY, AND
2. The run type is `RunTypeLong` (any task is long).

Can be forced with `run -ui=tui`.
