# 🏃🏽‍♀️🏃🏾‍♂️🏃🏻‍♀️💨 _⸻RUN_

<img alt="interactive TUI" src="https://github.com/amonks/run/blob/main/screenshots/tui.gif?raw=true" />

<img alt="noninteractive printed output" src="https://github.com/amonks/run/blob/main/screenshots/printer.gif?raw=true" />

```toml
# ./tasks.toml

[[task]]
  id = "build"
  type = "short"
  dependencies = ["css/build", "js/build"]
```

```toml
# ./css/tasks.toml

[[task]]
  id = "build"
  type = "short"
  watch = ["src.css"]
  cmd = """
    echo "Building CSS"
    build-css src.css > dist.css
    echo "done"
  """
```

_Find a full example configuration in the [example folder](https://github.com/amonks/run/tree/amonks/table/example)._

Run runs a collection of programs specified in tasks.toml files, and
provides a UI for inspecting their execution. Run's interactive UI for
long-lived programs has full mouse support.

Run also works well for short-lived processes, and its interleaved output
can be sent to a file.

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

Run can be used and extended programmatically through its Go API. See [the
godoc][godoc].

[godoc]: https://amonks.github.io/run

## Installation

Run is a single binary, which you can download from from the [releases
page][releases].

Alternately, if you already use go, you can install Run with the go command
line tool:

    $ go install github.com/amonks/run/cmd/run@latest

[releases]: https://github.com/amonks/run/releases

## Task Files

Task files are called "tasks.toml". They specify one or more tasks.

```toml
[[task]]
  id = "dev"
  type = "long"
  dependencies = ["simulate-coding"]
  triggers = ["build-css", "build-js"]
  watch = ["server-config.json"]
  env = {
    KEY="some value"
    OTHER_KEY="some other value"
  }
  cmd = """
    echo "dev-server running at http://localhost:3000"
    while true; do sleep 1; done
  """
```

There's an example project in the [example folder][example], where you can see
a realistic configuration.

[example]: https://github.com/amonks/run/tree/main/example

Let's go through the fields that can be specified on tasks.

### ID

ID identifies a task, for example,

- for command line invocation, as in `$ run <id>`
- in the TUI's task list.

### Type

Type specifies how we manage a task.

If the Type is "long",

- We will keep the task alive by restarting it if it exits.
- If the long task A is a dependency of task B, we will begin B as soon
  as A starts.
- It is invalid to use a long task as a trigger, since long tasks
  aren't expected to end.

If the Type is "short",

- If the Start returns nil, we will consider it done.
- If the Start returns an error, we will wait 1 second and rerun it.
- If the short task A is a dependency or trigger of task B, we will
  wait for A to complete before starting B.

Any Type besides "long", or "short" is invalid. There is no default
type: every task must specify its type.

### Dependencies

Dependencies are other tasks IDs which should always run alongside
this task. If a task A lists B as a dependency, running A will first
run B.

Dependencies do not set up an invalidation relationship: if long task
A lists short task B as a dependency, and B reruns because a watched
file is changed, we will not restart A, assuming that A has its own
mechanism for detecting file changes. If A does not have such a
mechanhism, use a trigger rather than a dependency.

Dependencies can be task IDs from child directories. For example, the
dependency "css/build" specifies the task with ID "build" in the tasks
file "./css/tasks.toml".

If a task depends on a "long" task, Run doesn't really know when the
long task has produced whatever output is depended on, so the
dependent is run 500ms after the long task starts.

### Triggers

Triggers are other task IDs which should always be run alongside this
task, and whose success should cause this task to re-execute. If a
task A lists B as a dependency, and both A and B are running,
successful execution of B will always trigger an execution of A.

Triggers can be task IDs from child directories. For example, the
trigger "css/build" specifies the task with ID "build" in the tasks
file "./css/tasks.toml".

It is invalid to use a "long" task as a trigger.

### Watch

Watch specifies file paths where, if a change to the file path is detected, we
should restart the task. Watch supports globs, and does **not** support the
"./..." style used typical of Go command line tools.

For example,

- `"."` watches for changes to the working directory only,
  but not changes within subdirectories.
- `"**"` watches for changes at any level within the
  working directory.
- `"./some/path/file.txt"` watches for changes to the file,
  which must already exist.
- `"./src/website/**/*.js"` watches for changes to
  javascript files within src/website.

### Env

Env is a map from environment variable keys to values. They are set for the
bash process that runs CMD.

### CMD

CMD is the command to run. It runs in a new bash process, as in,

    $ bash -c "$CMD"

CMD can have many lines.

## CLI Usage

    $ run dev

Run takes one argument: the task ID to run. Run looks for a task file in the current directory.

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

### User Interfaces

Run has two UIs that it deploys in different circumstances, a TUI and a
Printer. You can force Run to use a particular UI by passing the 'ui' flag,
as in,

    $ run -ui=printer dev

#### Interactive TUI

<img alt="interactive TUI" src="https://github.com/amonks/run/blob/main/screenshots/tui.gif?raw=true" />

The Interactive TUI is used whenever both,

1. stdout is a tty (eg Run is _not_ being piped to a file), and,
2. any running task is "long" (eg an ongoing "dev server" process rather than a
   one-shot "build" procedure).

For example, when running a dev server or test executor that stays running
while you make changes.

#### Non-Interactive Printer UI

| in your terminal...                                                                                                 | or as part of a pipeline...                                                                                   |
| ------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| <img alt="non-interactive output" src="https://github.com/amonks/run/blob/main/screenshots/printer.gif?raw=true" /> | <img alt="redirected output" src="https://github.com/amonks/run/blob/main/screenshots/nontty.gif?raw=true" /> |

Run prints its output if either,

1. run is not a tty (eg Run is being piped to a file), or,
2. no tasks are "long" (eg a one-shot "build" procedure, rather than an ongoing
   "dev server").

## Programmatic Use

Run can be used and extended programmatically through its Go API. For more
information, including a conceptual overview of the architecture, example code,
and reference documentation, see [the godoc][godoc].

[godoc]: https://amonks.github.io/run

# Attribution and License

Run is free for noncommercial and small-business use, with a guarantee that
fair, reasonable, and nondiscriminatory paid-license terms will be available
for large businesses. Ask about paid licenses at a@monks.co. See LICENSE.md or
invoke the program with `-license` for more details.

Run is made by Andrew Monks, with help from outside contributors. See
CONTRIBUTORS.md for more details.

Run makes use of a variety of open source software. See CREDITS.txt for more
details.
