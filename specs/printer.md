# Printer UI

## Overview

The non-interactive printer UI interleaves output from all tasks into a single stream, prefixing each line with a color-coded, right-aligned task ID. Suitable for CI, piped output, and short-only runs.

## Construction

```go
func New(run *runner.Run) runner.UI
```

Returns a `UI` that writes interleaved task output.

## Output Format

- Each line is prefixed with the task ID, right-aligned to the width of the longest task ID.
- Task IDs are color-coded using deterministic hash-based colors from `internal/color`.
- Consecutive lines from the same task omit the repeated ID prefix.
- A blank line separates output when the active task ID changes.
- Empty lines within task output are skipped.

## Writer

Each task ID gets a `printerWriter` that calls `printer.Write(id, content)`. The printer splits content on newlines and formats each line.

## Lifecycle

- `Start` records the output writer and computes key alignment width, then signals readiness.
- Blocks on context cancellation.
- Writes are thread-safe via internal mutex.

## UI Selection Logic

The printer is selected automatically when:

1. Stdout is not a TTY (piped output), OR
2. The run type is `RunTypeShort` (all tasks are short).

Can be forced with `run -ui=printer`.
