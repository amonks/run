# Watcher

## Overview

The `internal/watcher` package provides file system watching with debouncing and glob matching. It wraps the `notify` library and adds debouncing, glob-based filtering, and mock support for tests.

## Watch

```go
var Watch = func(inputPath string) (<-chan []EventInfo, func(), error) { ... }
```

A package-level function variable (to allow replacement via `Mock`).

- Accepts a path that may contain globs (e.g., `"src/website/**/*.js"`).
- Splits the path into a watch directory and an optional glob filter using `Split`.
- Returns a channel of debounced `[]EventInfo` batches, a `stop` function, and any error.
- Events are debounced with a 500ms window.
- Paths in emitted events are relative to the current working directory.

## EventInfo

```go
type EventInfo struct {
    Path  string
    Event string
}
```

Describes a single file system event.

## Split

```go
func Split(input string) (string, glob.Glob)
```

Breaks a path (possibly containing globs) into a watch path and a glob matcher.

- `"src/website/**/*.js"` → `("src/website/...", Glob)`
- `"src/website"` → `("src/website", nil)`
- `"."` → `(".", nil)`

The `"..."` suffix tells the underlying watcher to watch recursively.

## Debounce

```go
func Debounce(dur time.Duration, c <-chan EventInfo) <-chan []EventInfo
```

Collects individual events into batches, emitting each batch after `dur` of inactivity.

## Mock Support

### Mock

```go
func Mock() func()
```

Replaces `Watch` with a mock implementation and returns a restore function. While mocked:

- Calls to `Watch` register a channel keyed by the input path.
- Use `Dispatch` to send synthetic events to these channels.

### Dispatch

```go
func Dispatch(path string, evs ...EventInfo)
```

Sends synthetic events to the mock watcher for the given path. The path must match the `inputPath` used in a prior `Watch` call. No-op if no matching watcher exists.
