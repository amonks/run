package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
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
	Start(ctx context.Context, stdout io.Writer) error
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
	//   - We will restart the task if it returns.
	//   - If the long task A is a dependency or trigger of
	//     task B, we will begin B as soon as A starts.
	//
	// If the Type is "short",
	//   - If the Start returns nil, we will consider it done.
	//   - If the Start returns an error, we will wait 1 second and rerun it.
	//   - If the short task A is a dependency or trigger of task B, we will
	//     wait for A to complete before starting B.
	//
	// If the Type is "group",
	//   - We won't ever call task.Start.
	//   - For the purposes of invalidation, we will treat a group task as
	//     complete as soon as all of its dependencies are complete.
	//   - Groups define a collection of dependencies which can be used by
	//     other tasks. For example, imagine the group task Build, which
	//     depends on Build-Frontend and Build-Backend. Tasks like Install
	//     and Publish can depend on Build, and Build's definition can be
	//     updated in one place.
	//   - Groups can only have "dependencies", not "triggers" or "watch".
	//     It is invalid to have a group with no dependencies.
	//
	// Any Type besides "long", "short", or "group" is invalid. There is no
	// default type: every task must specify its type.
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

func (ts Tasks) IDs() []string {
	var ids []string
	for id := range ts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Validate inspects a set of Tasks and returns an error if
// the set is invalid. If the error is not nill, its
// [error.Error] will return a formatted multiline string
// describing the problems with the task set.
func (ts Tasks) Validate() error {
	var problems []string

	ids := map[string]struct{}{}
	for id, t := range ts {
		if id != t.Metadata().ID {
			problems = append(problems, fmt.Sprintf("- task '%s' has mismatched key '%s'", t.Metadata().ID, id))
		}
		ids[t.Metadata().ID] = struct{}{}
	}
	for _, t := range ts {
		for _, err := range ts.validateTask(ids, t) {
			problems = append(problems, "- "+err.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(append([]string{"invalid taskfile"}, problems...), "\n"))
	}
	return nil
}

func (ts Tasks) validateTask(ids map[string]struct{}, t Task) []error {
	var problems []error

	meta := t.Metadata()
	if meta.ID == "" {
		problems = append(problems, errors.New("Task has no ID."))
	}

	if meta.ID == "interleaved" || meta.ID == "run" {
		problems = append(problems, fmt.Errorf("'%s' is reserved and cannot be used as a task ID.", meta.ID))
	}

	for _, c := range meta.ID {
		if unicode.IsSpace(c) {
			problems = append(problems, fmt.Errorf("Task IDs cannot contain whitespace characters."))
		}
	}

	if meta.Type != "long" && meta.Type != "short" && meta.Type != "group" {
		problems = append(problems, fmt.Errorf("Task '%s' has invalid type '%s'; must be 'long', 'short', or 'group'.", meta.ID, meta.Type))
	}

	if meta.Type == "group" {
		if len(meta.Dependencies) == 0 {
			problems = append(problems, fmt.Errorf("Task '%s' is a group, but has no dependencies. Groups must include at least one dependency.", meta.ID))
		}
		if len(meta.Triggers) > 0 {
			problems = append(problems, fmt.Errorf("Task '%s' is a group, but has triggers. Groups may not have triggers.", meta.ID))
		}
		if len(meta.Watch) > 0 {
			problems = append(problems, fmt.Errorf("Task '%s' is a group, but has watch. Groups may not have watch.", meta.ID))
		}
		if s, isScript := t.(*scriptTask); isScript {
			if s.script != "" {
				problems = append(problems, fmt.Errorf("Task '%s' is a group, but has a cmd. The cmd will not be executed.", meta.ID))
			}
		}
	} else {
		if s, isScript := t.(*scriptTask); isScript {
			if s.script == "" {
				problems = append(problems, fmt.Errorf("Task '%s' is not a group, but has no cmd. It should be a group.", meta.ID))
			}
		}
	}

	for _, id := range meta.Dependencies {
		if _, ok := ids[id]; !ok {
			problems = append(problems, fmt.Errorf("Task '%s' lists dependency '%s', which is not the ID of a task.", meta.ID, id))
		}
	}

	for _, id := range meta.Triggers {
		if _, ok := ids[id]; !ok {
			problems = append(problems, fmt.Errorf("Task '%s' lists trigger '%s', which is not the ID of a task.", meta.ID, id))
		}
	}

	return problems
}
