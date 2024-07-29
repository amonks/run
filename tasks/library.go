package tasks

import (
	"sort"
)

// A Library is an opaque data structure representing an immutable, ordered
// collection of [Task]s.
type Library struct {
	ids   []string
	tasks map[string]Task

	watchset map[string]struct{}
}

// NewLibrary creates a Library with the given tasks in it.
func NewLibrary(tasks ...Task) Library {
	ts := Library{tasks: map[string]Task{}}
	for _, t := range tasks {
		id := t.Metadata().ID
		if _, isDuplicate := ts.tasks[id]; isDuplicate {
			continue
		}
		ts.ids = append(ts.ids, id)
		ts.tasks[id] = t
	}
	ts.materializeWatchset()
	return ts
}

// IDs returns, in order, the task IDs present in the Tasks.
func (lib Library) IDs() []string { return lib.ids }

// LongestID returns the width of the longest task ID in the library,
// _including_ internal task IDs used by the runner.
func (lib Library) LongestID() int {
	longest := len("@interleaved") // XXX: can't import the const here because it'd be a cycle
	for _, id := range lib.ids {
		if l := len(id); l > longest {
			longest = l
		}
	}
	return longest
}

// Task returns the task with the given ID, or nil if there is no such task.
func (lib Library) Task(id string) Task { return lib.tasks[id] }

// Size returns the number of unique tasks in the library.
func (lib Library) Size() int {
	return len(lib.ids)
}

// Has returns true if the library contains a task with the given ID.
func (lib Library) Has(id string) bool {
	_, has := lib.tasks[id]
	return has
}

// Watches returns, in alphabetical order, the complete set of file watches
// present among the Tasks. To find the watches implicated by a particular task
// and its dependencies, call Subtree first.
func (lib Library) Watches() []string {
	var watches []string
	for w := range lib.watchset {
		watches = append(watches, w)
	}
	sort.Strings(watches)
	return watches
}

// HasWatch returns true if the library contains a watcher on the given path.
func (lib Library) HasWatch(path string) bool {
	_, has := lib.watchset[path]
	return has
}

// Subtree returns a new Library containing only the given tasks and their
// dependencies, preserving the canonical ID order.
func (lib Library) Subtree(ids ...string) Library {
	include := map[string]struct{}{}
	stack := append([]string{}, ids...)
	for i := 0; i < len(stack); i++ {
		id := stack[i]
		t := lib.Task(id)
		if t == nil {
			continue
		}
		include[id] = struct{}{}
		stack = append(stack, t.Metadata().Dependencies...)
	}
	subtree := Library{tasks: map[string]Task{}}
	for _, id := range lib.ids {
		if _, isIncluded := include[id]; isIncluded {
			subtree.ids = append(subtree.ids, id)
			subtree.tasks[id] = lib.Task(id)
		}
	}
	subtree.materializeWatchset()
	return subtree
}

// WithWatch returns, in canonical order, the list of task IDs that watch the
// given path.
func (lib Library) WithWatch(watch string) []string {
	return lib.matches(func(t Task) bool {
		for _, w := range t.Metadata().Watch {
			if w == watch {
				return true
			}
		}
		return false
	})
}

// WithDependency returns, in canonical order, the list of task IDs that have
// the given dependency.
func (lib Library) WithDependency(dependency string) []string {
	return lib.matches(func(t Task) bool {
		for _, d := range t.Metadata().Dependencies {
			if d == dependency {
				return true
			}
		}
		return false
	})
}

// WithTrigger returns, in canonical order, the list of task IDs that have the
// given trigger.
func (lib Library) WithTrigger(trigger string) []string {
	return lib.matches(func(t Task) bool {
		for _, trig := range t.Metadata().Triggers {
			if trig == trigger {
				return true
			}
		}
		return false
	})
}

func (lib Library) HasAnyLongTask() bool {
	for _, t := range lib.tasks {
		if t.Metadata().Type == "long" {
			return true
		}
	}
	return false
}

func (lib Library) matches(pred func(Task) bool) []string {
	taskset := map[string]struct{}{}
	for id, t := range lib.tasks {
		if pred(t) {
			taskset[id] = struct{}{}
		}
	}
	var ids []string
	for _, id := range lib.ids {
		if _, isIncluded := taskset[id]; isIncluded {
			ids = append(ids, id)
		}
	}
	return ids
}

// materializeWatchset computes and stores lib.watchset. It is not threadsafe,
// so it must be called before handing a Library to the user.
func (lib *Library) materializeWatchset() {
	if lib.watchset != nil {
		return
	}
	watchset := map[string]struct{}{}
	for _, t := range lib.tasks {
		for _, w := range t.Metadata().Watch {
			watchset[w] = struct{}{}
		}
	}
	lib.watchset = watchset
}
