package runner

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
)

// Load loads a task file from the specified directory, producing a set of
// Tasks.
func Load(root string) (Tasks, error) {
	allTasks := map[string]TaskfileTask{}

	var ingestTaskMap func(dir string) error
	ingestTaskMap = func(dir string) error {
		relativeDir := strings.TrimPrefix(dir, root)
		relativeDir = strings.TrimPrefix(relativeDir, "/")
		if relativeDir == "" {
			relativeDir = "."
		}

		theseTasks, err := load(dir)
		if err != nil {
			return err
		}
		depSet := map[string]struct{}{}
		for _, t := range theseTasks {
			t := t.withDir(relativeDir)

			allTasks[t.ID] = t
			for _, dep := range t.Dependencies {
				if strings.Contains(dep, "/") {
					depSet[dep] = struct{}{}
				}
			}
			for _, dep := range t.Triggers {
				if strings.Contains(dep, "/") {
					depSet[dep] = struct{}{}
				}
			}
		}

		for id := range depSet {
			// ignore the task ID and just load the whole
			// referenced taskfile
			if err := ingestTaskMap(path.Join(dir, strings.TrimPrefix(path.Dir(id), relativeDir+"/"))); err != nil {
				return err
			}
		}

		return nil
	}

	if err := ingestTaskMap(root); err != nil {
		return nil, err
	}

	tf := make(Tasks, len(allTasks))
	for id, t := range allTasks {
		tf[id] = t.toCMDTask()
	}
	if err := tf.Validate(); err != nil {
		return nil, err
	}

	return tf, nil
}

func load(dir string) (map[string]TaskfileTask, error) {
	f, err := os.ReadFile(path.Join(dir, "tasks.toml"))
	if err != nil {
		return nil, err
	}
	var parsed Taskfile
	if err := toml.Unmarshal(f, &parsed); err != nil {
		return nil, err
	}
	taskMap := map[string]TaskfileTask{}
	for _, t := range parsed.Tasks {
		taskMap[t.ID] = t
	}
	return taskMap, nil
}

type Tasks map[string]Task

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

type Taskfile struct {
	Tasks []TaskfileTask `toml:"task"`
}

type TaskfileTask struct {
	// ID identifies a task, for example,
	// - for command line invocation, as in `$ runner <id>`
	// - in the TUI's task list.
	ID string `toml:"id"`

	// Type specifies how we manage a task.
	//
	// If the Type is "long",
	// - We will restart the task if it exits.
	// - If the long task A is a dependency or trigger of
	//   task B, we will begin B as soon as A starts.
	//
	// If the Type is "short",
	// - If the task exits 0, we will consider it done.
	// - If the task exits !0, we will wait 1 second and rerun it.
	// - If the short task A is a dependency or trigger of task B, we will
	//   wait for A to complete before starting B.
	//
	// Any Type besides "long" or "short" is invalid. There is no default
	// type: every task must specify its type.
	Type string `toml:"type"`

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
	Dependencies []string `toml:"dependencies"`

	// Triggers are other task IDs which should always be run alongside
	// this task, and whose success should cause this task to re-execute.
	// If a task A lists B as a dependency, and both A and B are running,
	// successful execution of B will always trigger an execution of A.
	//
	// Triggers can be task IDs from child directories. For example, the
	// trigger "css/build" specifies the task with ID "build" in the tasks
	// file "./css/tasks.toml".
	Triggers []string `toml:"triggers"`

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
	Watch []string `toml:"watch"`

	// CMD is the command to run. It runs in a new bash process, as in,
	//     $ bash -c "$CMD"
	// CMD can have many lines.
	CMD string `toml:"cmd"`

	dir string
}

func (t TaskfileTask) withDir(dir string) TaskfileTask {
	if dir == "." {
		return t
	}
	t.ID = path.Join(dir, t.ID)
	for i, dep := range t.Dependencies {
		t.Dependencies[i] = path.Join(dir, dep)
	}
	for i, dep := range t.Triggers {
		t.Triggers[i] = path.Join(dir, dep)
	}
	return t
}

func (t TaskfileTask) toCMDTask() Task {
	return ScriptTask(t.CMD, TaskMetadata{
		ID:           t.ID,
		Type:         t.Type,
		Dependencies: t.Dependencies,
		Triggers:     t.Triggers,
		Watch:        t.Watch,
		CWD:          t.dir,
	})
}
