# Task Engine

## Overview

The task engine is the core of Run. It loads task definitions, validates them, resolves dependencies, schedules execution, manages file watchers, and coordinates task lifecycles. The public API lives in `pkg/run`.

## Core Types

### Task (interface)

Anything implementing `Task` can be executed by the engine.

```go
type Task interface {
    Start(ctx context.Context, stdout io.Writer) error
    Metadata() TaskMetadata
}
```

- `Start` runs the task, writing output to `stdout`. It blocks until the task completes or the context is canceled.
- `Metadata` returns the task's configuration.
- Implementations must be safe for concurrent access from multiple goroutines.

### TaskMetadata

```go
type TaskMetadata struct {
    ID           string
    Description  string
    Type         string    // "long" or "short"
    Dependencies []string
    Triggers     []string
    Watch        []string
}
```

### Tasks

An opaque, immutable, ordered collection of tasks.

- `NewTasks([]Task) Tasks` creates a collection from a slice.
- `IDs() []string` returns task IDs in canonical order.
- `Has(id string) bool` checks for a task by ID.
- `Get(id string) Task` looks up a task by ID (nil if absent).
- `Validate() error` returns a multiline error describing any problems.

## Task Implementations

### ScriptTask

Runs a bash script in a subprocess.

```go
func ScriptTask(script string, dir string, env []string, metadata TaskMetadata) Task
```

- Executes via `bash -c "$script"` in a new process group (`Setpgid: true`).
- Stdout and stderr are merged and forwarded to the provided writer.
- No stdin is provided.
- Environment inherits `os.Environ()` plus the provided `env` entries.
- On context cancellation: sends SIGINT to the process group, waits up to 2 seconds, then sends SIGKILL.
- A task with no script: long tasks block until context cancellation; short tasks return immediately.
- Bash is located once via `which bash` and cached for the process lifetime.

### FuncTask

Runs a Go function.

```go
func FuncTask(fn func(ctx context.Context, w io.Writer) error, metadata TaskMetadata) Task
```

- Runs the function in a goroutine and waits for completion or context cancellation.
- Simpler alternative to `ScriptTask` for programmatic use.

## Run

A `Run` represents the execution of a task and all its transitive dependencies, triggers, and watches.

```go
func RunTask(dir string, allTasks Tasks, taskID string) (*Run, error)
```

- Validates the task set before proceeding.
- Recursively ingests the requested task and all reachable tasks via dependencies and triggers.
- Preserves the canonical ordering from `allTasks`.
- Determines `RunType`: `RunTypeLong` if the root task is long, `RunTypeShort` otherwise.

### RunType

- `RunTypeShort`: exits once the root task succeeds, or immediately on any task failure.
- `RunTypeLong`: runs until context cancellation. File watches are only active in long runs.

### TaskStatus

Tracks per-task state for UI rendering only; never affects control flow.

- `TaskStatusNotStarted`
- `TaskStatusRunning`
- `TaskStatusRestarting`
- `TaskStatusFailed`
- `TaskStatusDone`

### Execution

`Run.Start(ctx context.Context, out MultiWriter) error` starts execution:

1. Sets up file watchers for all watched paths.
2. Starts all zero-dependency tasks concurrently.
3. Enters the main event loop, handling:
   - **File system events**: debounced, matched against globs, trigger task invalidation.
   - **Start requests**: cancel any running instance of the task, then restart it.
   - **Task readiness**: when a task becomes ready (short task completes, or long task runs for 500ms), start dependent and triggered tasks whose dependencies are all met.
   - **Task exits**: update status. In short runs, exit on root task completion or any failure. In long runs, retry failed tasks with exponential backoff (1s, 2s, 4s, … capped at 30s); restart long tasks as keepalive. Manual restarts (via `Invalidate`) and file-watch restarts reset the backoff counter.
   - **Context cancellation**: return nil.
4. On exit, stops all file watchers, cancels all running tasks, and waits for them to finish.

### Invalidation

`Run.Invalidate(id string)` externally requests a task to rerun. Only applies to tasks that are running, done, or failed.

### MultiWriter (interface)

```go
type MultiWriter interface {
    Writer(id string) io.Writer
}
```

The output interface that `Run.Start` requires. Both `TUI` and `Printer` implement it.

### Internal Task IDs

- `@interleaved`: used by the TUI for its interleaved output pane.
- `@watch`: used for file watcher status messages.

## Output Writer

Task output passes through a `lineBufferedWriter` (flushes on newline) and a `jsonWriter` (pretty-prints any valid JSON). This ensures JSON output from tasks is human-readable in the UI.

## Validation Rules

- Task IDs must be non-empty.
- Task IDs cannot contain whitespace or `@` (reserved).
- Task type must be `"long"` or `"short"` (`"group"` is deprecated).
- Dependencies and triggers must reference existing tasks.
- Triggers cannot reference long tasks.
- Watch paths must be relative and within the working directory.
