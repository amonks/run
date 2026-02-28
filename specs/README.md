# Specifications

## Architecture

### System Principles

- The specifications are the source of truth for system behavior; update them alongside code changes.
- Run combines task sequencing (like Make or Task) with process management (like Overmind or Foreman) in a single tool.
- The public API lives in `pkg/run` and `pkg/logview`; internal packages under `internal/` are not part of the public contract.
- All public types must be safe for concurrent access from multiple goroutines.
- Composition over inheritance: the `Task` and `UI` interfaces allow custom implementations.

### Packages

| Spec | Code | Purpose |
| --- | --- | --- |
| [task-engine.md](./task-engine.md) | [pkg/run/](../pkg/run/) | Core task engine: loading, validation, dependency resolution, scheduling, and execution |
| [taskfile.md](./taskfile.md) | [pkg/run/taskfile.go](../pkg/run/taskfile.go) | TOML task file format, loading, and monorepo support |
| [tui.md](./tui.md) | [pkg/run/tui*.go](../pkg/run/) | Interactive terminal UI built on BubbleTea |
| [printer.md](./printer.md) | [pkg/run/printer.go](../pkg/run/printer.go) | Non-interactive printer UI for CI and piped output |
| [logview.md](./logview.md) | [pkg/logview/](../pkg/logview/) | Reusable BubbleTea log viewer component |
| [cli.md](./cli.md) | [cmd/run/](../cmd/run/) | CLI entry point, flag parsing, and UI selection |
| [scroll.md](./scroll.md) | [cmd/scroll/](../cmd/scroll/) | Standalone tailing log pager |
| [testing.md](./testing.md) | | Testing patterns and snapshot tests |

### Internal Packages

| Spec | Code | Purpose |
| --- | --- | --- |
| [internal-color.md](./internal-color.md) | [internal/color/](../internal/color/) | Deterministic hash-based color generation and Solarized palette |
| [internal-help.md](./internal-help.md) | [internal/help/](../internal/help/) | Help menu rendering with section/key layout |
| [internal-mutex.md](./internal-mutex.md) | [internal/mutex/](../internal/mutex/) | Debug-capable mutex with defer-friendly API |
