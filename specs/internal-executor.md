# Executor

## Overview

The `internal/executor` package encapsulates the lifecycle of a single cancelable function execution: running it, canceling it, and waiting for it to finish. It is used by the task engine to manage per-task goroutines.

## Core Type

### Executor

```go
type Executor struct { /* unexported fields */ }
```

- Created via `New()`, which assigns a unique token for identity comparison.
- Inert until `Execute` is called.

### Construction

```go
func New() *Executor
```

Returns an `Executor` with a unique identity token and an open `Done` channel.

### Execute

```go
func (e *Executor) Execute(ctx context.Context, fn func(context.Context) error)
```

- Derives a cancelable child context from `ctx`.
- Runs `fn` in a new goroutine.
- When `fn` returns, stores its error and closes the `Done` channel.
- Must be called exactly once per `Executor`.

### Cancel

```go
func (e *Executor) Cancel() error
```

- Cancels the derived context and **blocks** until `fn` exits.
- Returns `fn`'s error.
- Idempotent: safe to call multiple times or concurrently.

### Done

```go
func (e *Executor) Done() <-chan struct{}
```

Returns a channel that is closed when `fn` exits. Multiple goroutines can `select` on this channel simultaneously.

### Err

```go
func (e *Executor) Err() error
```

Returns `fn`'s error. Only valid after `Done()` is closed.

### Is

```go
func (e *Executor) Is(other *Executor) bool
```

Returns true if `other` is the same `Executor` instance (by token comparison). Used for stale-goroutine detection: when a task-exit message arrives, the caller checks whether the executor that exited is still the current one for that task.

## Design Notes

- `Done()` returns a closable channel (not a value channel) so multiple goroutines can select on it.
- The unique token uses an `atomic.Int64` counter, avoiding the need for pointer comparison.
- `Cancel` is synchronous by design — the caller knows the function has fully exited before proceeding.
