package taskfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amonks/run/tasks"
	"github.com/amonks/run/tasks/script"
)

// Taskfile defines the type of tasks.toml files. You can load one from disk,
// or, if you want, you can create your own in code.
type Taskfile struct {
	Tasks []Task `toml:"task"`
}

// Load loads a task file from the specified directory, loads any referenced
// taskfiles from subdirectories, and combines them into a single Taskfile.
func Load(cwd string) (Taskfile, error) {
	var allTasks []Task

	seenDirs := map[string]struct{}{}
	var ingestTaskMap func(dir string) error
	ingestTaskMap = func(dir string) error {
		if _, ok := seenDirs[dir]; ok {
			return nil
		}
		seenDirs[dir] = struct{}{}

		relativeDir := strings.TrimPrefix(dir, "/")
		if relativeDir == "" {
			relativeDir = "."
		}

		theseTasks, err := load(cwd, dir)
		if err != nil {
			return err
		}
		depSet := map[string]struct{}{}
		for _, t := range theseTasks {
			t := t.withDir(cwd, relativeDir)

			allTasks = append(allTasks, t)
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

		// Reload referenced taskfiles (if there are any)
		for id := range depSet {
			p := filepath.Dir(id)
			if p == relativeDir {
				continue
			}
			// ignore the task ID and just load the whole
			// referenced taskfile

			p = strings.TrimPrefix(p, relativeDir+string(os.PathSeparator))
			p = filepath.Join(dir, p)
			if err := ingestTaskMap(p); err != nil {
				return err
			}
		}

		return nil
	}

	if err := ingestTaskMap("."); err != nil {
		return Taskfile{}, err
	}

	return Taskfile{allTasks}, nil
}

// TODO
func (Taskfile) Validate() error {
	return nil
}

func (tf Taskfile) ToLibrary() tasks.Library {
	ts := make([]tasks.Task, len(tf.Tasks))
	for i, t := range tf.Tasks {
		ts[i] = t.ToScriptTask()
	}
	return tasks.NewLibrary(ts...)
}

func (tf Taskfile) find(id string) Task {
	for _, t := range tf.Tasks {
		if t.ID == id {
			return t
		}
	}
	return Task{}
}

func load(cwd, dir string) ([]Task, error) {
	f, err := os.ReadFile(filepath.Join(cwd, dir, "tasks.toml"))
	if err != nil {
		return nil, err
	}
	var parsed Taskfile
	if err := toml.Unmarshal(f, &parsed); err != nil {
		return nil, err
	}
	return parsed.Tasks, nil
}

type Task struct {
	ID           string            `toml:"id"`
	Description  string            `toml:"description"`
	Type         string            `toml:"type"`
	Dependencies []string          `toml:"dependencies"`
	Triggers     []string          `toml:"triggers"`
	Watch        []string          `toml:"watch"`
	CMD          string            `toml:"cmd"`
	Env          map[string]string `toml:"env"`

	// Dir specifies the relative path to the directory containing this
	// taskfile.
	Dir string
}

func (t Task) ToScriptTask() script.Task {
	description := t.Description
	if description == "" && t.CMD != "" && !strings.Contains(t.CMD, "\n") {
		description = fmt.Sprintf(`"%s"`, t.CMD)
	}
	metadata := tasks.TaskMetadata{
		ID:           t.ID,
		Description:  description,
		Type:         t.Type,
		Dependencies: t.Dependencies,
		Triggers:     t.Triggers,
		Watch:        t.Watch,
	}
	return script.New(metadata, t.Dir, t.Env, t.CMD)
}

func (t Task) withDir(cwd, dir string) Task {
	t.ID = filepath.Join(dir, filepath.FromSlash(t.ID))
	t.Dir = filepath.Join(cwd, dir)
	for i, dep := range t.Dependencies {
		t.Dependencies[i] = filepath.Join(dir, dep)
	}
	for i, dep := range t.Triggers {
		t.Triggers[i] = filepath.Join(dir, dep)
	}
	for i, p := range t.Watch {
		t.Watch[i] = filepath.Join(dir, p)
	}
	return t
}
