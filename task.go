package run

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Tasks is a map from IDs to Tasks. The string keys of the map must match the
// TaskMetadata.ID of each associated Task. You can create a [Run] by passing
// Tasks into [RunTask].
type Tasks map[string]Task

// Anything implementing Task can be run by bundling it into a [Tasks] and then
// passing it into [RunTask].
//
// [ScriptTask] and [FuncTask] can be used to create Tasks.
//
// A Task must be safe to access concurrently from multiple goroutines.
type Task interface {
	Start(stdout io.Writer) error
	Wait() <-chan error
	Stop() error
	Metadata() TaskMetadata
}

// TaskMetadata contains the data which, regardless of the type of Task, a
// [Run] uses for task execution.
type TaskMetadata struct {
	// ID identifies a task, for example,
	//   - for command line invocation, as in `$ run <id>`
	//   - in the TUI's task list.
	ID string

	// Type specifies how we manage a task.
	//
	// If the Type is "long",
	//   - We will restart the task if it exits.
	//   - If the long task A is a dependency or trigger of
	//     task B, we will begin B as soon as A starts.
	//
	// If the Type is "short",
	//   - If the task exits 0, we will consider it done.
	//   - If the task exits !0, we will wait 1 second and rerun it.
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
	// such a mechanhism, use a trigger rather than a dependency.
	//
	// Dependencies can be task IDs from child directories. For example,
	// the dependency "css/build" specifies the task with ID "build" in the
	// tasks file "./css/tasks.toml".
	Dependencies []string

	// Triggers are other task IDs which should always be run alongside
	// this task, and whose success should cause this task to re-execute.
	// If a task A lists B as a dependency, and both A and B are running,
	// successful execution of B will always trigger an execution of A.
	//
	// Triggers can be task IDs from child directories. For example, the
	// trigger "css/build" specifies the task with ID "build" in the tasks
	// file "./css/tasks.toml".
	Triggers []string

	// Watch specifies file paths where, if a change to the file path is
	// detected, we should restart the task. Recursive paths are specified
	// with the suffix "/...".
	//
	// For example,
	// - "." watches for changes to the working directory only, but not
	//   changes within subdirectories.
	// - "./..." watches for changes at any level within the working
	//   directory.
	// - "./some/path/file.txt" watches for changes to the file, which may
	//   or may not already exist.
	Watch []string
}

// Validate inspects a set of Tasks and returns an error if
// the set is invalid. If the error is not nill, its
// [error.Error] will return a formatted multiline string
// describing the problems with the task set.
func (tf Tasks) Validate() error {
	ids := map[string]struct{}{}
	for _, t := range tf {
		ids[t.Metadata().ID] = struct{}{}
	}
	var problems []string
	for _, t := range tf {
		for _, err := range tf.validateTask(ids, t) {
			problems = append(problems, "- "+err.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(append([]string{"invalid taskfile"}, problems...), "\n"))
	}
	return nil
}

func (tf Tasks) validateTask(ids map[string]struct{}, t Task) []error {
	var problems []error

	meta := t.Metadata()
	if meta.ID == "" {
		problems = append(problems, errors.New("task has no ID"))
	}

	if meta.ID == "interleaved" || meta.ID == "run" {
		problems = append(problems, fmt.Errorf("'%s' is reserved and cannot be used as a task ID", meta.ID))
	}

	for _, c := range meta.ID {
		if unicode.IsSpace(c) {
			problems = append(problems, fmt.Errorf("task IDs cannot contain whitespace characters"))
		}
	}

	if meta.Type != "long" && meta.Type != "short" {
		problems = append(problems, fmt.Errorf("task %s has invalid type '%s', must be 'long' or 'short'", meta.ID, meta.Type))
	}

	for _, id := range meta.Dependencies {
		if _, ok := ids[id]; !ok {
			problems = append(problems, fmt.Errorf("task %s lists dependency '%s', which is not the ID of a task", meta.ID, id))
		}
	}

	for _, id := range meta.Triggers {
		if _, ok := ids[id]; !ok {
			problems = append(problems, fmt.Errorf("task %s lists trigger '%s', which is not the ID of a task", meta.ID, id))
		}
	}

	return problems
}
