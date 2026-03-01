# Script

## Overview

The `internal/script` package provides an immutable, reentrant wrapper around `exec.Cmd` for running bash scripts with robust cancellation. It is used by `ScriptTask` in `pkg/run` to execute shell commands.

## Core Type

### Script

```go
type Script struct {
    Dir  string
    Env  []string
    Text string
}
```

An immutable value type describing a bash script to execute. Safe to copy, compare, and start multiple times concurrently.

- `Dir`: working directory for the script. If empty, uses the current directory.
- `Env`: additional environment variables in `KEY=VALUE` format, appended to `os.Environ()`.
- `Text`: the bash script text, passed to `bash -c`.

### Start

```go
func (s Script) Start(ctx context.Context, stdout, stderr io.Writer) error
```

Executes the script in a new bash process and blocks until completion or context cancellation. Each call creates a fresh process (`execution`), so multiple Starts can run concurrently on the same Script value.

- Creates a new process group (`Setpgid: true`) for signal management.
- Stdout and stderr are wired to the provided writers independently.
- Bash is located once via `which bash` and cached for the process lifetime.
- Returns nil only if the process exits with code 0 without context cancellation.

## Cancellation

On context cancellation:

1. Sends SIGINT to the process group.
2. Waits up to 2 seconds for the process to exit gracefully.
3. If still running, sends SIGKILL.
4. Returns an error that includes `context.Canceled`.

## Design Notes

- **Immutable value type**: `Script` has no mutable state. All mutable process state (`*exec.Cmd`) lives in the internal `execution` struct, created fresh per `Start` call.
- **Reentrant**: the same `Script` can be started multiple times, including concurrently, because each `Start` creates an independent `execution`.
- **Separate stdout/stderr**: unlike the public `ScriptTask` (which merges them), the `Script.Start` API accepts separate writers, enabling independent capture in tests.
- **Process-group signals**: SIGINT and SIGKILL are sent to the negative PID (process group), ensuring child processes are also terminated.
