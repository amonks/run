package run

import "sort"

// Library is an opaque data structure representing an immutable, ordered
// collection of [Task]s. You can create a [Run] by passing a Library into
// [RunTask].
type Library struct {
	ids      []string
	tasks    map[string]Task
	watchset map[string]struct{}
}

// NewLibrary creates a Library from the given tasks.
func NewLibrary(tasks ...Task) Library {
	lib := Library{
		ids:   make([]string, len(tasks)),
		tasks: make(map[string]Task, len(tasks)),
	}
	for i, t := range tasks {
		id := t.Metadata().ID
		lib.ids[i] = id
		lib.tasks[id] = t
	}
	lib.watchset = lib.materializeWatchset()
	return lib
}

func (lib Library) materializeWatchset() map[string]struct{} {
	ws := map[string]struct{}{}
	for _, id := range lib.ids {
		for _, w := range lib.tasks[id].Metadata().Watch {
			ws[w] = struct{}{}
		}
	}
	return ws
}

// IDs returns the task IDs in their canonical order.
func (lib Library) IDs() []string {
	return lib.ids
}

// Has returns true if the given ID is present among the Library.
func (lib Library) Has(id string) bool {
	_, ok := lib.tasks[id]
	return ok
}

// Get looks up a specific task by its ID. If no task bearing that ID is
// present, the task will be nil.
func (lib Library) Get(id string) Task {
	return lib.tasks[id]
}

// Size returns the number of tasks in the Library.
func (lib Library) Size() int {
	return len(lib.ids)
}

// LongestID returns the length of the longest task ID, including internal
// IDs like @interleaved.
func (lib Library) LongestID() int {
	longest := len(internalTaskInterleaved)
	for _, id := range lib.ids {
		if len(id) > longest {
			longest = len(id)
		}
	}
	return longest
}

// Watches returns a sorted slice of unique watched paths across all tasks.
func (lib Library) Watches() []string {
	ps := make([]string, 0, len(lib.watchset))
	for p := range lib.watchset {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	return ps
}

// HasWatch returns true if any task in the Library watches the given path.
func (lib Library) HasWatch(path string) bool {
	_, ok := lib.watchset[path]
	return ok
}

// Subtree returns a new Library containing only the given task IDs and their
// transitive dependencies and triggers, preserving the canonical order from
// the original Library.
func (lib Library) Subtree(ids ...string) Library {
	included := map[string]struct{}{}
	var walk func(string)
	walk = func(id string) {
		if _, ok := included[id]; ok {
			return
		}
		if !lib.Has(id) {
			return
		}
		included[id] = struct{}{}
		t := lib.tasks[id]
		for _, d := range t.Metadata().Dependencies {
			walk(d)
		}
		for _, d := range t.Metadata().Triggers {
			walk(d)
		}
	}
	for _, id := range ids {
		walk(id)
	}

	// Preserve canonical order.
	var orderedIDs []string
	var orderedTasks []Task
	for _, id := range lib.ids {
		if _, ok := included[id]; ok {
			orderedIDs = append(orderedIDs, id)
			orderedTasks = append(orderedTasks, lib.tasks[id])
		}
	}

	sub := Library{
		ids:   orderedIDs,
		tasks: make(map[string]Task, len(orderedIDs)),
	}
	for _, t := range orderedTasks {
		sub.tasks[t.Metadata().ID] = t
	}
	sub.watchset = sub.materializeWatchset()
	return sub
}

// WithWatch returns the IDs of tasks that watch the given path.
func (lib Library) WithWatch(path string) []string {
	return lib.matches(func(t Task) bool {
		for _, w := range t.Metadata().Watch {
			if w == path {
				return true
			}
		}
		return false
	})
}

// WithDependency returns the IDs of tasks that list dep as a dependency.
func (lib Library) WithDependency(dep string) []string {
	return lib.matches(func(t Task) bool {
		for _, d := range t.Metadata().Dependencies {
			if d == dep {
				return true
			}
		}
		return false
	})
}

// WithTrigger returns the IDs of tasks that list trigger as a trigger.
func (lib Library) WithTrigger(trigger string) []string {
	return lib.matches(func(t Task) bool {
		for _, d := range t.Metadata().Triggers {
			if d == trigger {
				return true
			}
		}
		return false
	})
}

func (lib Library) matches(pred func(Task) bool) []string {
	var out []string
	for _, id := range lib.ids {
		if pred(lib.tasks[id]) {
			out = append(out, id)
		}
	}
	return out
}

// Validate inspects a Library and returns an error if
// it is invalid. If the error is not nil, its
// [error.Error] will return a formatted multiline string
// describing the problems with the task set.
func (lib Library) Validate() error {
	return newValidator().validate(lib)
}
