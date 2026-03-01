package run

import (
	"context"
	"io"
)

// Anything implementing Task can be run by bundling it into a [Library] and then
// passing it into [RunTask].
//
// [ScriptTask] and [FuncTask] can be used to create Tasks.
//
// A Task must be safe to access concurrently from multiple goroutines.
type Task interface {
	// Start runs the task. It should write output to stdout and close
	// onReady when the task is ready (i.e., has produced whatever output
	// dependents need). For short tasks, close onReady on successful
	// completion. For long tasks, close onReady as soon as the task is
	// serving / ready.
	Start(ctx context.Context, onReady chan<- struct{}, stdout io.Writer) error
	Metadata() TaskMetadata
}

// TaskMetadata contains the data which, regardless of the type of Task, a
// [Run] uses for task execution.
type TaskMetadata struct {
	// ID identifies a task, for example,
	//   - for command line invocation, as in `$ run <id>`
	//   - in the TUI's task list.
	ID string

	// Description optionally provides additional information about a task,
	// which can be displayed, for example, by running `run -list`. It can
	// be one line or many lines.
	Description string

	// Type specifies how we manage a task.
	//
	// If the Type is "long",
	//   - We will keep the task alive by restarting it if it exits.
	//   - If the long task A is a dependency of task B, we will begin B as
	//     soon as A starts.
	//   - It is invalid to use a long task as a trigger, since long tasks
	//     aren't expected to end.
	//
	// If the Type is "short",
	//   - If the Start returns nil, we will consider it done.
	//   - If the Start returns an error, we will wait 1 second and rerun it.
	//   - If the short task A is a dependency or trigger of task B, we will
	//     wait for A to complete before starting B.
	//
	// Any Type besides "long" or "short" is invalid. There is no default
	// type: every task must specify its type.
	Type string

	// Dependencies are other tasks IDs which should always run alongside
	// this task. If a task A lists B as a dependency, running A will first
	// run B.
	//
	// Dependencies do not set up an invalidation relationship: if long
	// task A lists short task B as a dependency, and B reruns because a
	// watched file is changed, we will not restart A, assuming that A has
	// its own mechanism for detecting file changes. If A does not have
	// such a mechanism, use a trigger rather than a dependency.
	//
	// Dependencies can be task IDs from child directories. For example,
	// the dependency "css/build" specifies the task with ID "build" in the
	// tasks file "./css/tasks.toml".
	//
	// If a task depends on a "long" task, the dependent is started once
	// the long task signals readiness by closing its onReady channel.
	Dependencies []string

	// Triggers are other task IDs which should always be run alongside
	// this task, and whose success should cause this task to re-execute.
	// If a task A lists B as a dependency, and both A and B are running,
	// successful execution of B will always trigger an execution of A.
	//
	// Triggers can be task IDs from child directories. For example, the
	// trigger "css/build" specifies the task with ID "build" in the tasks
	// file "./css/tasks.toml".
	//
	// It is invalid to use a "long" task as a trigger.
	Triggers []string

	// Watch specifies file paths where, if a change to
	// the file path is detected, we should restart the
	// task. Watch supports globs, and does **not**
	// support the "./..." style used typical of Go
	// command line tools.
	//
	// For example,
	//  - `"."` watches for changes to the working
	//    directory only, but not changes within
	//    subdirectories.
	//  - `"**" watches for changes at any level within
	//    the working directory.
	//  - `"./some/path/file.txt"` watches for changes
	//    to the file, which must already exist.
	//  - `"./src/website/**/*.js"` watches for changes
	//    to javascript files within src/website.
	Watch []string
}
