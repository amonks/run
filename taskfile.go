package run

import (
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
)

// Load loads a task file from the specified directory, producing a set of
// Tasks.
func Load(root string) (Tasks, error) {
	allTasks := map[string]taskfileTask{}

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

func load(dir string) (map[string]taskfileTask, error) {
	f, err := os.ReadFile(path.Join(dir, "tasks.toml"))
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
	ID string `toml:"id"`
	Type string `toml:"type"`
	Dependencies []string `toml:"dependencies"`
	Triggers []string `toml:"triggers"`
	Watch []string `toml:"watch"`
	// CMD is the command to run. It runs in a new bash process, as in,
	//     $ bash -c "$CMD"
	// CMD can have many lines.
	CMD string `toml:"cmd"`

	dir string
}

func (t taskfileTask) withDir(dir string) taskfileTask {
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

func (t taskfileTask) toCMDTask() Task {
	return ScriptTask(t.CMD, t.dir, TaskMetadata{
		ID:           t.ID,
		Type:         t.Type,
		Dependencies: t.Dependencies,
		Triggers:     t.Triggers,
		Watch:        t.Watch,
	})
}
