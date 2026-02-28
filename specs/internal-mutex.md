# Internal Mutex

## Overview

The `internal/mutex` package wraps `sync.Mutex` with a defer-friendly API and optional debug logging.

## API

```go
func New(name string) *Mutex
func (mu *Mutex) Lock(name string) *Mutex
func (mu *Mutex) Unlock()
```

## Defer Pattern

Supports single-line lock/unlock:

```go
defer mu.Lock("context").Unlock()
```

`Lock` returns the mutex itself, allowing chained `Unlock` in a defer statement.

## Debug Logging

- Controlled by the `debug` constant (compile-time, default `false`).
- When enabled, writes timestamped lock/unlock events to `mutex.log`.
- Log entries include: mutex name, operation, and current holder.
- Uses a separate internal `sync.Mutex` to protect the holder field and log writes.

## Usage

Used throughout the codebase for thread-safe access:
- `Run` coordination
- `scriptTask` process management
- `tui` and `printer` write operations
- `lineBufferedWriter` output buffering
