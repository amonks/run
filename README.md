# 🏃🏽‍♀️🏃🏾‍♂️🏃🏻‍♀️💨 **_⸻ RUN_**
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/amonks/run?logo=go&logoColor=white&label=%20&labelColor=gray&color=00ADD8)
[![Godoc](https://img.shields.io/badge/go-docs-blue?logo=go&logoColor=white&label=%20&labelColor=gray&color=blue)](https://amonks.github.io/run)
[![Go Report Card](https://goreportcard.com/badge/github.com/amonks/run)](https://goreportcard.com/report/github.com/amonks/run)

Run is a task runner that simplifies executing and managing tasks defined in `tasks.toml` files. It provides a versatile set of features making it well suited for a range of use cases, from simple build scripts to complex development workflows.

<img alt="interactive TUI" src="screenshots/tui.gif?raw=true" />

## Features

- **Flexible Task Configuration**: Support for complex task dependencies, environment variable injection, and file watching for automatic task re-execution.
- **Interactive TUI**: Full mouse support for managing long-lived tasks.
- **Non-Interactive Output**: Interleaved output suitable for short-lived processes.
- **Go API**: Extensibility through a Go programming interface.

## Installation

Run can be installed as a single binary or via the Go command line tool:

### Pre-compiled binary

Download the latest release from the [releases page](https://github.com/amonks/run/releases), and extract the binary to a directory in your `PATH`. This can be done in a single command like:

    $ curl -sL https://github.com/amonks/run/releases/download/<RELEASE_VERSION>/run_<RELEASE_ARCH>.tar.gz | tar -x && chmod +x run && mv run ~/go/bin

### Using Go

If you already use go and have it installed, you can install Run with the go command line tool.

    $ go install github.com/amonks/run/cmd/run@latest

## Quick Start

Follow these steps to get started with Run:

1. **Create a `tasks.toml` file**
    
        $ touch tasks.toml

2. **Define a task in that `tasks.toml`**
    ```toml
    [[task]]
      id = "hello"
      type = "short"
      cmd = "echo Hello, World!"
    ```

3. **Run the task and see task output**
      
    <img alt="hello-gif" src="screenshots/hello.gif">

## Configuration

Run is configured through tasks.toml files. Here's a brief overview of the key fields in a task definition:

_Required Fields_:

- `id`: Unique identifier for the task.
- `type`: Specifies the task type (long or short).

_Optional Fields_:

- `dependencies`: Other tasks to run alongside this task.
- `triggers`: Tasks that, when completed successfully, will cause this task to re-execute.
- `watch`: File paths to monitor for changes, triggering task restarts.
- `env`: Environment variables for the task's execution context.
- `cmd`: The command to run, may contain multiple lines.

For explanations about task fields and behavior head to the [Task Configuration](#task-configuration-details) section, or check out the [examples](#examples) for practical use cases.

## CLI Usage

Run a task simply by specifying its ID:

    $ run <task-id>

Available flags are:
<!-- usage-start -->
```
USAGE
     
  run [flags] <task>

     
FLAGS
     
  -contributors
        Display the contributors list and exit.
  -credits
        Display the open source credits and exit.
  -dir=string (default ".")
        Look for a root taskfile in the given directory.
  -help
        Display the help text and exit.
  -license
        Display the license info and exit.
  -list
        Display the task list and exit. If run is invoked
        with both -list and a task ID, that task's
        dependencies are displayed.
  -ui=string
        Force a particular ui. Legal values are 'tui' and
        'printer'.
  -version
        Display the version and exit.
```
<!-- usage-end -->

## User Interfaces

Run has two UIs that it deploys in different circumstances, a TUI and a
Printer. You can force Run to use a particular UI by passing the 'ui' flag,
as in,

    $ run -ui=printer dev

### Interactive TUI

<img alt="interactive TUI" src="https://github.com/amonks/run/blob/main/screenshots/tui.gif?raw=true" />

The Interactive TUI is used whenever both,

1. stdout is a tty (eg Run is _not_ being piped to a file), and,
2. any running task is "long" (eg an ongoing "dev server" process rather than a
   one-shot "build" procedure).

For example, when running a dev server or test executor that stays running
while you make changes.

### Non-Interactive Printer UI

| in your terminal...                                                                                                 | or as part of a pipeline...                                                                                   |
| ------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| <img alt="non-interactive output" src="https://github.com/amonks/run/blob/main/screenshots/printer.gif?raw=true" /> | <img alt="redirected output" src="https://github.com/amonks/run/blob/main/screenshots/nontty.gif?raw=true" /> |

Run prints its output if either,

1. run is not a tty (eg Run is being piped to a file), or,
2. no tasks are "long" (eg a one-shot "build" procedure, rather than an ongoing
   "dev server").

## Task Configuration Details

The core of Run's configuration is the `tasks.toml` file, which defines the tasks you want Run to manage. Each task is specified with a range of properties, as shown in the example below:

```toml
[[task]]
  id = "dev"
  type = "long"
  dependencies = ["simulate-coding"]
  triggers = ["build-css", "build-js"]
  watch = ["server-config.json"]
  env = {
    KEY = "some value"
    OTHER_KEY = "some other value"
  }
  cmd = """
    echo "dev-server running at http://localhost:3000"
    while true; do sleep 1; done
  """
```

Explore a [realistic project configuration](https://github.com/amonks/run/tree/main/examples/demo) to see `tasks.toml` in action.

### Understanding Task Fields

#### Required Fields

Task definitions have two required fields: `id` and `type`. _Note_: if only the required fields are set and none of `dependencies`, `triggers`, or `cmd` are specified, then the task is a no-op.

##### `id`

The unique identifier for a task. It is used for:
- Command line invocation (`$ run <id>`)
- Identifying the task in the interactive UI (TUI).

##### `type`

Specifies the task's lifecycle management strategy:
- A task can be of only one type: `long` or `short`, any other value is invalid
- **`long`**: The task is kept alive indefinitely. It's restarted if it exits unexpectedly. **Not suitable as a trigger**.
  - For example, a development server or a test runner.
  - If a task depends on a "long" task, Run doesn't really know when the long task has produced whatever output is depended on, so the dependent is run 500ms after the long task starts.
- **`short`**: The task is considered complete upon successful execution. If it fails, Run will retry it after a short delay.
  - For example, a build script or a code generation task.

#### Optional Fields

##### `description`

A human-readable description of the task. It's displayed in the TUI task list, the output of `run -list`, and can be used to document the task's purpose. It can be one line or multiline.

##### `dependencies`
Lists other task IDs that should run alongside this task. 

- If task A depends on task B, B starts before A.
- If B is a "long" task, A will start 500ms after B starts, and if B is a "short" task, A will start as soon as B completes.[^](#type)
  
##### `triggers`
Lists other task IDs that should run alongside this task, and when completed successfully, cause this task to restart. 

- Triggers are similar to dependencies but specifically for re-execution upon successful completion of the trigger task.
- If task A is triggered by task B, and both A and B are running, a successful completion of B will trigger A to restart.
- **Triggers are not considered when a task is initially run.** They only affect task restarts.
- **Long tasks cannot be triggers.** It is invalid to use a long task as a trigger, since long tasks aren't expected to end.

>[!NOTE]
> Task IDs listed as `dependencies` and `triggers` can cross directory boundaries (e.g., `"css/build"` refers to a task in `./css/tasks.toml`).

##### `watch`
Defines file paths or globs to monitor for changes. Any detected change triggers a task restart. Examples include:
- `"."` for the current directory (excluding subdirectories)
- `"**"` for any change within the working directory
- Specific paths like `"./src/website/**/*.js"` for targeted file types.

##### `env`
A map of environment variables provided to the task's execution environment. 

- These variables are available to the command running the task.
- They are appended to the current environment, overriding any existing variables with the same name.
- They persist for the duration of the task's execution.

#### `cmd`
The shell command to execute as the task. It's run in a new bash process:

    $ bash -c "$CMD"

In simple cases, the command can be a single line:

```toml
[[task]]
  id = "clean"
  type = "short"
  cmd = "go clean -testcache && go clean -modcache"
```

For more complex commands, or to aid readability, use a multiline string:

```toml
[[task]]
  id = "clean"
  type = "short"
  cmd = """
    echo "Cleaning up..."
    set -x

    rm -rf ./bin
    go clean -testcache
    go clean -modcache
  """
```

## Programmatic Use

Run can be used and extended programmatically through its Go API. 

In this small example, we use components from Run to build our own version of the run CLI tool. See cmd/run for the source of the -real- run CLI, which isn't too much more complex. 

```go
package main

import "github.com/amonks/run/pkg/run"

func main() {
	tasks, _ := run.Load(".")
	r, _ := run.RunTask(".", tasks, "dev")
	ui := run.NewTUI(r)

	ctx := context.Background()
	uiReady := make(chan struct{})

	go ui.Start(ctx, uiReady, os.Stdin, os.Stdout)
	<- uiReady

	r.Start(ctx, ui) // blocks until done
}
```

For more information, a conceptual overview of the architecture, example code, and reference documentation, see [the godoc][godoc].

[godoc]: https://amonks.github.io/run

## Examples

- [demo](https://github.com/amonks/run/tree/main/examples/demo): A realistic project configuration with multiple tasks. Used to demonstrate Run's capabilities.
- [hello](https://github.com/amonks/run/tree/main/examples/hello): A simple "Hello, World!" task file to get started with Run.
- [golang-app](https://github.com/amonks/run/tree/main/examples/golang-app): A template Golang project with tasks for bootstrapping a new project, building, testing, and running the application.


## Attribution and License

Run is free for noncommercial and small-business use, with a guarantee that
fair, reasonable, and nondiscriminatory paid-license terms will be available
for large businesses. Ask about paid licenses at a@monks.co. See LICENSE.md or
invoke the program with `-license` for more details.

Run is made by Andrew Monks, with help from outside contributors. See
CONTRIBUTORS.md for more details.

Run makes use of a variety of open source software. See CREDITS.txt for more
details.