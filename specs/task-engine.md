# Task Engine

## Overview

The task engine is the core of Run. It loads task definitions, validates them, resolves dependencies, schedules execution, manages file watchers, and coordinates task lifecycles. The public API lives in `pkg/run`.

## Core Types

### Task (interface)

Anything implementing `Task` can be executed by the engine.

```go
type Task interface {
    Start(ctx context.Context, onReady chan<- struct{}, stdout io.Writer) error
    Metadata() TaskMetadata
}
```

- `Start` runs the task, writing output to `stdout`. It blocks until the task completes or the context is canceled.
- `onReady` must be closed by the task when it is ready (i.e., has produced whatever output dependents need). This replaces the previous 500ms heuristic.
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

### Library

An opaque, immutable, ordered collection of tasks with reverse-query methods.

- `NewLibrary(tasks ...Task) Library` creates a collection from variadic tasks.
- `IDs() []string` returns task IDs in canonical order.
- `Has(id string) bool` checks for a task by ID.
- `Get(id string) Task` looks up a task by ID (nil if absent).
- `Size() int` returns the number of tasks.
- `LongestID() int` returns the length of the longest task ID (including internal IDs like `@interleaved`).
- `Watches() []string` returns a sorted slice of unique watched paths across all tasks.
- `HasWatch(path string) bool` returns true if any task watches the given path.
- `Subtree(ids ...string) Library` returns a new Library containing only the given task IDs and their transitive dependencies and triggers, preserving canonical order.
- `WithWatch(path string) []string` returns task IDs that watch the given path.
- `WithDependency(dep string) []string` returns task IDs that list `dep` as a dependency.
- `WithTrigger(trigger string) []string` returns task IDs that list `trigger` as a trigger.
- `Validate() error` returns a multiline error describing any problems.

Internally, a Library materializes a watchset (`map[string]struct{}`) at construction time for efficient `Watches()`, `HasWatch()`, and `WithWatch()` queries.

## Task Implementations

### ScriptTask

Runs a bash script in a subprocess. Delegates to `internal/script.Script` for process management.

```go
func ScriptTask(script string, dir string, env []string, metadata TaskMetadata) Task
```

- Delegates execution to `internal/script.Script`, which handles process lifecycle and cancellation.
- Stdout and stderr are merged and forwarded to the provided writer.
- No stdin is provided.
- A task with no script: closes `onReady` immediately; long tasks block until context cancellation; short tasks return immediately.
- A long task with a script: closes `onReady` before execution starts.
- A short task with a script: closes `onReady` on successful exit (nil error).
- `scriptTask` is stateless (no mutex, no mutable fields); all mutable process state lives in `internal/script`.

### FuncTask

Runs a Go function.

```go
func FuncTask(fn func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error, metadata TaskMetadata) Task
```

- Runs the function in a goroutine and waits for completion or context cancellation.
- The function receives an `onReady` channel that it must close when it is ready.
- Simpler alternative to `ScriptTask` for programmatic use.

## Run

A `Run` represents the execution of a task and all its transitive dependencies, triggers, and watches.

```go
func RunTask(dir string, allTasks Library, taskID string) (*Run, error)
```

- Validates the task set before proceeding.
- Uses `allTasks.Subtree(taskID)` to compute the active task set from the requested task and all reachable tasks via dependencies and triggers.
- Preserves the canonical ordering from `allTasks`.
- Determines `RunType`: `RunTypeLong` if the root task is long, `RunTypeShort` otherwise.
- Tracks requested task IDs in a `requestedTasks` set for dynamic recomputation.

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

1. Sets up output writers.
2. Starts file watchers for all watched paths (via `r.tasks.Watches()`).
3. Sends `msgRunTask` for each zero-dependency task to the input channel.
4. Enters the single-channel event loop, dispatching messages from `r.input`:
   - **`msgRunTask`**: cancels any existing executor for the task (synchronously via `executor.Cancel()`), creates a new `Executor`, starts the task with an `onReady` channel, sets up a readiness listener goroutine (selects on `onReady` vs `exec.Done()`) and an exit-forwarding goroutine.
   - **`msgTaskReady`**: marks the task as "ran", uses `r.tasks.WithTrigger(id)` and `r.tasks.WithDependency(id)` to find and start eligible dependents and triggered tasks.
   - **`msgTaskExit`**: discards if the task's current executor is nil (task was removed) or does not match the exiting executor (`Executor.Is()` for stale detection). Otherwise, updates status. In short runs, exits on root task completion or any failure. In long runs, retries failed tasks with exponential backoff (1s, 2s, 4s, … capped at 30s); restarts long tasks as keepalive.
   - **`msgFSEvent`**: uses `r.tasks.WithWatch(path)` to find affected tasks, resets backoff, sends `msgRunTask` for them.
   - **`msgInvalidate`**: resets backoff, sends `msgRunTask` for the task (if running, done, or failed).
   - **Context cancellation**: exits the loop with nil error.
5. On exit, stops all file watchers, cancels all executors (blocking until each exits), and returns.

Each task is managed by an `internal/executor.Executor` which encapsulates the cancelable goroutine lifecycle. Stale exit messages (from a previous incarnation of a task) are detected via `Executor.Is()` and discarded.

All mutable state (`taskStatus`, `restartAttempts`, `ran`, `executors`, `writers`) is stored in plain maps guarded by a single `mutex.Mutex` (`r.mu`). Handlers never send to `r.input` while holding `r.mu`.

### Invalidation

`Run.Invalidate(id string)` sends a `msgInvalidate` to the event loop. Only applies to tasks that are running, done, or failed.

### Dynamic Task Management

Tasks can be dynamically added to or removed from a running `Run`. The `Run` maintains a `requestedTasks` set tracking explicitly requested root task IDs. Dynamic operations modify this set and recompute the active task subtree using `Library.Subtree()`.

#### Add

```go
func (r *Run) Add(ids ...string)
```

Sends a `msgAddTasks` message to the event loop. The handler:

1. Adds new IDs to `requestedTasks`.
2. Recomputes the active task set via `r.allTasks.Subtree(allRequestedIDs...)`.
3. For tasks in the new set but not the old: sets up writers, task status, and starts zero-dep tasks.
4. Starts file watchers for any new watch paths (via `Library.Watches()`).

Task IDs that are already active or not found in `allTasks` are silently ignored.

#### Remove

```go
func (r *Run) Remove(id string)
```

Sends a `msgRemoveTask` message to the event loop. The handler:

1. Removes `id` from `requestedTasks`.
2. Recomputes the active task set via `r.allTasks.Subtree(remainingRequestedIDs...)`, explicitly excluding the removed task even if it is a transitive dependency of another requested task.
3. For tasks in the old set but not the new: cancels executors, cleans up status maps.
4. Stops file watchers for paths no longer watched (comparing old vs new `Watches()`).

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
