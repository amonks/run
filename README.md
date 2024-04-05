# 🏃🏽‍♀️🏃🏾‍♂️🏃🏻‍♀️💨 _⸻RUN_

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/amonks/run?logo=go&logoColor=white&label=%20&labelColor=gray&color=00ADD8)
[![Godoc](https://img.shields.io/badge/go-docs-blue?logo=go&logoColor=white&label=%20&labelColor=gray&color=blue)](https://amonks.github.io/run)

| <img alt="non-interactive output" src="https://github.com/amonks/run/blob/main/screenshots/printer.gif?raw=true" /> | <img alt="interactive TUI" src="https://github.com/amonks/run/blob/main/screenshots/tui.gif?raw=true" /> |
| ------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |

Run is a dependency-aware task runner with an interactive process management
UI.

Task runners (like [GNU Make][gnu make], [Task][go-task], or [Gulp][gulpjs])
are great for task sequencing, but they aren't suitable for managing the
long-running programs that make up a dev environment or a production service.
Process managers (like [Overmind][overmind], [Foreman][foreman], or [Exo][exo])
are great for managing long-running programs, but they aren't suitable for
sequencing the dependent tasks that make up a build or test process. Run
combines the best features from both worlds.

- **Flexible Task Configuration**: Configure tasks with complex dependency
  chains, filesystem triggers, and environment variable injection.
- **Interactive TUI**: Search and inspect task logs and even restart tasks
  interactively with full mouse support.
- **Non-Interactive CI Mode**: Share task configuration and dependency logic
  between your dev environment and your CI/CD system.
- **Go API**: Use Run's task engine and UI components to build your own custom
  tools.

[gnu make]: https://www.gnu.org/software/make/
[go-task]: https://github.com/go-task/task
[gulpjs]: https://gulpjs.com
[overmind]: https://github.com/DarthSim/overmind
[foreman]: https://github.com/ddollar/foreman
[exo]: https://exo.deref.io

# Installation

Run is a single binary, which you can download from from the [releases page][releases].

## Install Script (MacOS, Linux)

This install command will download the latest version of run to your current directory.

```sh
curl -L https://raw.githubusercontent.com/amonks/run/main/install.bash | bash
```

## Install with Go

If you already use go, you can install Run with the go command line tool:

    $ go install github.com/amonks/run/cmd/run@latest

[releases]: https://github.com/amonks/run/releases

# Minimal Example

Here's an example task file you can play with. Scroll down to learn more about
integrating Run with your projects.

```toml
# tasks.toml

[[task]]
  id = "read-example-file"
  type = "long"
  dependencies = ["touch-example-file"]
  cmd = """
    while true; do
      echo "the example file says: $(cat example-file)"
      sleep 1
    done
  """

[[task]]
  id = "touch-example-file"
  type = "short"
  cmd = """
    date > example-file
  """
```

Run the tasks with `run read-example-file`. Select the `touch-example-file`
task and use `r` to restart it. See how the output of `read-example-file`
changes when this happens.

# Getting Started

Every codebase is different, but here's a general overview of how to integrate
Run into an existing project.

Depending on your learning style, you might prefer perusing the [example
project][example].

### 1. Consider your tasks

Make a list of the scripts and tasks you use to operate your project, making
groups of tasks that are run together.

- Testing
  - linting (eslint, govet, ...)
  - formatting (prettier, gofmt, ...)
  - running automated tests (jest, go test, ...)
- Building
  - compiling a compiled language (sass or postcss or tailwind, typescript,
    go, c++, ...)
  - code generation (go generate, yesql, ...)
  - bundling (webpack, parcel, browserify, vite, ...)
- Running in development
  - launching your project (npm start, go run, ...)
  - any auxilary tasks that your project requires (a database, a proxy, ...)

### 2. Write a task file

Create a file `tasks.toml` with "dev", "test", and "build" tasks. Here's a
basic template:

```toml
# tasks.toml

[[task]]
  id = "dev"
  type = "long"
  dependencies = [] # TODO

[[task]]
  id = "test"
  type = "short"
  dependencies = [] # TODO

[[task]]
  id = "build"
  type = "short"
  dependencies = [] # TODO
```

### 3. Add your tasks

For each task you identified in step 1, add a new task to your taskfile. For example,

```toml
[[task]]
  id = "lint"
  type = "short"
  cmd = "npm run lint"
```

Then, add the task to the appropriate group,

```diff
 [[task]]
   id = "test"
   type = "short"
-  dependencies = [] # TODO
+  dependencies = ["lint"]
```

### 4. Enjoy!

Run `run -list` to check your work. Then, run `run build`, `run dev`, or `run
test` to try it!

See [the example project][example] for a working demo.

[example]: https://github.com/amonks/run/tree/main/example

# Configuration

Run's configuration file is called `tasks.toml`. Here's a brief overview of the
key fields in a task definition:

### Required Fields

- `id`: Unique identifier for the task.
- `type`: Specifies the task type (long or short).

### Optional Fields

- `dependencies`: Other tasks to run alongside this task.
- `triggers`: Tasks that, when completed successfully, will cause this task to
  restart.
- `watch`: File paths to monitor for changes, which will cause this task to
  restart.
- `env`: Environment variables for the task's execution context.
- `cmd`: The command to run. This can be a multiline script.

For complete documentation, see the Taskfile Reference section below.

# Taskfile Reference

Run is configured with `tasks.toml` files, which define the tasks that Run will
manage.

Task definitions have two required fields: `id` and `type`.

> [!TIP]
> It is possible to define a no-op task that specifies `dependencies` but not
> `cmd`. This can be useful for making a task which groups other tasks
> together.

### `id` (required)

ID is a unique (within this taskfile) identifier for a task. It is used for:

- Command line invocation (`$ run <id>`)
- Identifying the task in the interactive UI (TUI).

### `type` (required)

Type specifies the task's lifecycle management strategy. The only valid values
are `long` and `short`:

- **`long`**: The task is kept alive indefinitely. It's restarted if it exits
  unexpectedly. **Not suitable as a trigger**.
  - For example, a development server or a test runner.
  - If a task depends on a "long" task, Run doesn't really know when the long
    task has produced whatever output is depended on, so the dependent is run
    500ms after the long task starts.
- **`short`**: The task is considered complete upon successful execution. If it
  fails, Run will retry it after a short delay.
  - For example, a build script or a code generation task.

### `description`

Description is a human-readable description of the task. It's displayed in the
TUI task list, the output of `run -list`, and can be used to document the
task's purpose. It can be one line or multiline.

### `dependencies`

Dependencies is a list of other task references that should run alongside this
task.

- If task A depends on task B, B starts before A.
- If B is a "long" task, A will start 500ms after B starts, and if B is a
  "short" task, A will start as soon as B completes.
- A task reference can be an ID from the current taskfile, or it can include a
  path to a child taskfile. See [Task References][#task-references] for more
  details.

### `triggers`

Triggers is a list of task references to "short" tasks that should run
alongside this task, and when completed successfully, cause this task to
restart.

- Triggers are similar to dependencies but specifically for re-execution upon
  successful completion of the trigger task.
- If task A is triggered by task B, and both A and B are running, a successful
  completion of B will trigger A to restart.
- **Triggers are not considered when a task is initially run.** They only
  affect task restarts.
- **Long tasks cannot be triggers.** It is invalid to use a long task as a
  trigger, since long tasks aren't expected to end.
- A task reference can be an ID from the current taskfile, or it can include a
  path to a child taskfile. See [Task References][#task-references] for more
  details.

> [!IMPORTANT]
> The difference between triggers and dependencies is a bit subtle. In general,
> "triggers" should always be "short", and "dependencies" should always be
> "long". Here are some examples,
>
> - a test runner with a `--watch` mode might be a "long" "dependency" of a
>   "dev" task. This way, the test runner is started once and kept running, and
>   test runs do not cause the dev server to restart.
> - a CSS builder with a `--watch` mode might be a "long" "dependency" of a
>   "dev" task, and its output file "style.css" might additionally be a
>   `watch`. This way, the css builder is run once and kept running, and when
>   "style.css" changes, the dev server will be restarted.
> - a CSS builder _without_ a `--watch` mode might be a "short" "trigger" of a
>   "dev" task, and it might list the input css files under "watch". This way,
>   the css builder is run whenever the input css files change, and its
>   successful execution triggers a restart of the dev server.

### `watch`

Watch defines file paths or globs to monitor for changes. Any detected change
triggers a task restart. Examples include:

- `"."` will watch files in the current directory (excluding subdirectories)
- `"*"` is a wildcard within a single path segment: eg "dist/\*.js" matches
  "dist/main.js" but not "dist/another/folder/main.js". "dist/\*" matches all
  files in "dist", but not any files in "dist/another/folder/".
- `"**"` is a wildcard that can span across zero or more path segments: eg
  "dist/\*\*/\*.js" matches "dist/main.js" and also
  "dist/another/folder/main.js"

### `env`

Env defines a map of environment variables provided to the task's execution
environment.

- The environment variables are available to the task's cmd script.
- The environment variables are _not_ available to any other task -- they are
  not inherited by dependencies, for example.
- They are appended to the current environment, overriding any existing
  variables with the same name.

### `cmd`

CMD is a shell script that defines what the task _does_. It's run in a new bash
process, as in:

    $ bash -c "$CMD"

In simple cases, the command can be a single line:

```toml
[[task]]
  id = "clean"
  type = "short"
  cmd = "go clean -testcache && go clean -modcache"
```

For more complex commands, or to aid readability, a multiline string is also
acceptable (note the triple-quote):

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

## Task References

Run supports "monorepo" use cases: a task file can reference tasks from child
directories with the `/` character. This is supported by all fields that take
task ID references: `dependencies` and `triggers`. Everything before the last
`/` is treated as a relative path to a directory containing a taskfile. The
string following the last `/` is treated as a task ID in that taskfile. This is
easy to understand with a few examples:

- `dependencies = ["build"]` refers to the task with ID "build" in
  ./tasks.toml.
- `dependencies = ["some/dir/build"]` refers to the task with ID "build" in
  ./some/dir/tasks.toml. It is equivalent to `cd some/dir && run build`

Only references to child directories are supported. `../build` is not a valid
task reference.

<table>
  <thead>
    <tr>
      <th>tasks.toml</th>
      <th>css/tasks.toml</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>
        <pre>[[task]]
  id = "build-css"
  dependencies = ["css/build"]</pre>
      </td>
      <td>
        <pre>[[task]]
  id = "build"
  cmd = "npx postcss build"</pre>
      </td>
    </tr>
  </tbody>
</table>

In the project structure above, "build-css" depends on the "build" task from
"css/tasks.toml".

# CLI Reference

    $ run dev

Run takes one argument: the task ID to run. Run looks for a task file in the
current directory.

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

# User Interfaces

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

1. stdout is not a tty (eg Run is being piped to a file), or,
2. no tasks are "long" (eg a one-shot "build" procedure, rather than an ongoing
   "dev server").

# Programmatic Use

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
