# Printer UI

## Overview

The non-interactive printer UI interleaves output from all tasks into a single stream, prefixing each line with a color-coded, right-aligned task ID. Suitable for CI, piped output, and short-only runs.

## Construction

```go
func New(gutterWidth int, stdout io.Writer) *Printer
```

Creates a `Printer` with a fixed gutter width (typically the length of the longest task ID) and an output writer. The `Printer` implements `runner.MultiWriter` and `runner.UI`.

## Output Format

- Each line is prefixed with the task ID, right-aligned to the gutter width.
- Task IDs are color-coded using deterministic hash-based colors from `internal/color`.
- Consecutive lines from the same task omit the repeated ID prefix.
- A blank line separates output when the active task ID changes.
- Empty lines within task output are skipped.

## Writer

Each task ID gets a `printerWriter` that calls `printer.write(id, content)`. The printer splits content on newlines and formats each line.

## Lifecycle

- `New` sets up the gutter width and output writer at construction time.
- `Start` (implementing `runner.UI`) signals readiness immediately and blocks on context cancellation.
- Writes are thread-safe via internal mutex.

## UI Selection Logic

The printer is selected automatically when:

1. Stdout is not a TTY (piped output), OR
2. The root task type is `"short"`.

Can be forced with `run -ui=printer`.
