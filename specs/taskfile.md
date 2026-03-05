# Task File Format

## Overview

Run is configured with `tasks.toml` files using TOML syntax. Each file defines an array of task tables.

## File Structure

```toml
[[task]]
  id = "task-id"                    # Required: unique identifier
  type = "short"                    # Required: "long" or "short"
  description = "..."               # Optional: shown in -list and TUI
  cmd = "script or command"         # Optional: bash script
  dependencies = ["dep1", "dep2"]   # Optional: tasks that run first
  triggers = ["trigger1"]           # Optional: short tasks that restart this
  watch = ["src/**/*.go"]           # Optional: file patterns to watch

  [task.env]                        # Optional: environment variables
    VAR = "value"
```

## Loading

`Load(cwd string, targetTaskIDs ...string) (Tasks, error)` loads and validates tasks from a directory:

1. Reads `tasks.toml` in the given directory.
2. For each task, resolves its directory relative to `cwd`.
3. If dependencies or triggers reference child directories (contain `/`), recursively loads those task files.
4. If any `targetTaskIDs` contain `/`, eagerly loads the taskfile in the target's directory (if it exists), even if no existing task references it. Missing directories are silently ignored.
5. Validates the combined task set.
6. Returns an error if the root file is missing, malformed, or invalid.

## Task Types

### Short

- Runs once. Success on exit code 0.
- On failure: retried after 1 second (in long runs).
- Dependent tasks wait for completion before starting.
- Can be used as triggers.

### Long

- Kept alive indefinitely; restarted automatically if it exits.
- Dependent tasks start 500ms after this task starts (heuristic; no readiness signal).
- Cannot be used as triggers.

## Dependencies vs Triggers

- **Dependencies** run alongside the task. If B is a dependency of A, B starts before A. Completion of B does not cause A to restart.
- **Triggers** also run alongside the task, but successful completion of a trigger causes the parent task to restart. Only short tasks can be triggers.

## Watch Patterns

Glob syntax for file watching:

- `"."` watches the current directory only (not subdirectories).
- `"*"` matches within a single path segment.
- `"**"` spans zero or more path segments.
- Example: `"src/**/*.js"` matches `src/main.js` and `src/nested/dir/main.js`.

Watch paths must be relative and within the working directory.

## Task References (Monorepo Support)

Dependencies and triggers support cross-directory references using `/`:

- `"build"` refers to task `build` in the current `tasks.toml`.
- `"css/build"` refers to task `build` in `./css/tasks.toml`.
- Only child directories are supported; `"../build"` is invalid.

When a cross-directory reference is encountered, Run automatically loads the referenced `tasks.toml`. Task IDs are prefixed with their relative directory path (e.g., `css/build`).

## Environment Variables

- `env` entries are appended to the current process environment.
- They override existing variables with the same name.
- They are scoped to the task's script only; not inherited by dependencies.

## Auto-Generated Description

If a task has no `description` and its `cmd` is a single line, the command is used as the description (wrapped in quotes).

## CMD Execution

The `cmd` field runs in a new bash process:

```
bash -c "$CMD"
```

Multi-line scripts are supported via TOML triple-quoted strings.
