# Interactive TUI

## Overview

The interactive TUI provides a split-pane terminal interface for viewing task output, built on the BubbleTea framework. It is used when stdout is a TTY and any running task is long.

## Construction

```go
func New(run *runner.Run) runner.UI
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
- Status spinners: `Jump` spinner for short tasks, `MiniDot` spinner for long tasks.
- Status indicators: ` ` (not started), spinner (running/restarting), `✓` (done), `×` (failed).
- Internal tasks (prefixed with `@`) show a blank spinner.

### Task List Scrolling

When there are more tasks than fit in the menu's available height, the task list scrolls to keep the selected task visible:

- If all tasks fit within `menuHeight`, they are rendered without any scroll indicators.
- If `menuHeight < 3`, a window around the selected task is shown without indicators (not enough room).
- Otherwise, the view shows a window of tasks with `▲ N` / `▼ N` indicator lines (dim `color.XDark`) when tasks are hidden above or below. Each indicator consumes one line of menu height.
- The scroll offset (`menuScrollOffset`) persists across renders and is only adjusted when the selected task would be outside the visible window, preventing unnecessary jumping.
- Navigation keys (j/k/gg/G/0-9) only change `selectedTaskIDIndex`; the scroll offset is adjusted on the next render cycle.

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

## Resize Handling

When a `tea.WindowSizeMsg` is received, the TUI propagates the new dimensions to all logview models via `SetDimensions()`, ensuring scroll calculations (page up/down, half-page scroll) use the correct viewport size.

## Program Configuration

- Alt screen and mouse cell motion are set via `tea.View` fields (BubbleTea v2 pattern), not `tea.NewProgram` options.
- 120 FPS rendering.
- Zone-based click detection via `bubblezone`.

## Styling

Styles are computed dynamically based on terminal width, height, and current focus area. The TUI uses Solarized-derived colors from `internal/color`.

## UI Selection Logic

The TUI is selected automatically when:

1. Stdout is a TTY, AND
2. The run type is `RunTypeLong` (any task is long).

Can be forced with `run -ui=tui`.
