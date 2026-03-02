# Specifications

## Architecture

### System Principles

- The specifications are the source of truth for system behavior; update them alongside code changes.
- Run combines task sequencing (like Make or Task) with process management (like Overmind or Foreman) in a single tool.
- The public API lives in `task/`, `taskfile/`, `runner/`, `tui/`, `printer/`, and `logview/`; internal packages under `internal/` are not part of the public contract.
- All public types must be safe for concurrent access from multiple goroutines.
- Composition over inheritance: the `Task` and `UI` interfaces allow custom implementations.

### Packages

| Spec | Code | Purpose |
| --- | --- | --- |
| [task-engine.md](./task-engine.md) | [task/](../task/), [runner/](../runner/) | Task definition, validation, dependency resolution, scheduling, and execution |
| [taskfile.md](./taskfile.md) | [taskfile/](../taskfile/) | TOML task file format, loading, and monorepo support |
| [tui.md](./tui.md) | [tui/](../tui/) | Interactive terminal UI built on BubbleTea |
| [printer.md](./printer.md) | [printer/](../printer/) | Non-interactive printer UI for CI and piped output |
| [logview.md](./logview.md) | [logview/](../logview/) | Reusable BubbleTea log viewer component |
| [cli.md](./cli.md) | [.](../) | CLI entry point, flag parsing, and UI selection |
| [scroll.md](./scroll.md) | [cmd/scroll/](../cmd/scroll/) | Standalone tailing log pager |
| [testing.md](./testing.md) | | Testing patterns and snapshot tests |

### Internal Packages

| Spec | Code | Purpose |
| --- | --- | --- |
| [internal-color.md](./internal-color.md) | [internal/color/](../internal/color/) | Deterministic hash-based color generation and Solarized palette |
| [internal-executor.md](./internal-executor.md) | [internal/executor/](../internal/executor/) | Cancelable function executor with identity tokens |
| [internal-help.md](./internal-help.md) | [internal/help/](../internal/help/) | Help menu rendering with section/key layout |
| [internal-mutex.md](./internal-mutex.md) | [internal/mutex/](../internal/mutex/) | Debug-capable mutex with defer-friendly API |
| [internal-watcher.md](./internal-watcher.md) | [internal/watcher/](../internal/watcher/) | File system watching with debouncing, globs, and mock support |
| [internal-script.md](./internal-script.md) | [internal/script/](../internal/script/) | Immutable, reentrant bash script executor with robust cancellation |
| | [internal/fixtures/](../internal/fixtures/) | Test doubles: mock Task and recording MultiWriter |
| | [internal/seq/](../internal/seq/) | Sequence assertion helper for ordered subsequence checks |
