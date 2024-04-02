package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Load loads a task file from the specified directory, producing a set of
// Tasks.
func Load(cwd string) (Tasks, error) {
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

			allTasks = append(allTasks, t.toScriptTask())
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
		return Tasks{}, err
	}

	tf := NewTasks(allTasks)

	v, err := newValidatorWithCWD(cwd)
	if err != nil {
		return Tasks{}, err
	}
	if err := v.validate(tf); err != nil {
		return Tasks{}, err
	}

	return tf, nil
}

func load(cwd, dir string) (map[string]taskfileTask, error) {
	f, err := os.ReadFile(filepath.Join(cwd, dir, "tasks.toml"))
	if err != nil {
		return nil, err
	}
	var parsed taskfile
	if err := toml.Unmarshal(f, &parsed); err != nil {
		return nil, err
	}
	taskMap := map[string]taskfileTask{}
	for _, t := range parsed.Tasks {
		taskMap[t.ID] = t
	}
	return taskMap, nil
}

type taskfile struct {
	Tasks []taskfileTask `toml:"task"`
}

type taskfileTask struct {
	ID           string   `toml:"id"`
	Description  string   `toml:"description"`
	Type         string   `toml:"type"`
	Dependencies []string `toml:"dependencies"`
	Triggers     []string `toml:"triggers"`
	Watch        []string `toml:"watch"`

	// CMD is the command to run. It runs in a new bash process, as in,
	//     $ bash -c "$CMD"
	// CMD can have many lines.
	CMD string `toml:"cmd"`

	// Env is a map of environment variables that are set for this task's
	// CMD process.
	Env map[string]string `toml:"env"`

	dir string
}

func (t taskfileTask) withDir(cwd, dir string) taskfileTask {
	t.ID = filepath.Join(dir, filepath.FromSlash(t.ID))
	t.dir = filepath.Join(cwd, dir)
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

func (t taskfileTask) toScriptTask() Task {
	description := t.Description
	if description == "" && t.CMD != "" && !strings.Contains(t.CMD, "\n") {
		description = fmt.Sprintf(`"%s"`, t.CMD)
	}
	var env []string
	for k, v := range t.Env {
		env = append(env, k+"="+v)
	}
	return ScriptTask(t.CMD, t.dir, env, TaskMetadata{
		ID:           t.ID,
		Description:  description,
		Type:         t.Type,
		Dependencies: t.Dependencies,
		Triggers:     t.Triggers,
		Watch:        t.Watch,
	})
}
