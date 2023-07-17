package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RunTask creates an executable Run from a taskList and a taskID.
//
// The run will handle task dependencies, watches, and triggers as documented
// in the README.
func RunTask(dir string, allTasks Tasks, taskID string) (*Run, error) {
	if err := allTasks.Validate(); err != nil {
		return nil, err
	}

	runType := RunTypeShort
	tasks := map[string]Task{}
	byDep := map[string][]string{}
	byTrigger := map[string][]string{}
	byWatch := map[string][]string{}

	taskStatus := newSafeMap[TaskStatus]()

	var ingestTask func(string) error
	ingestTask = func(id string) error {
		t, ok := allTasks[id]
		if !ok {
			lines := []string{fmt.Sprintf("Task %s not found. Tasks are,", id)}
			for id := range allTasks {
				lines = append(lines, "- "+id)
			}
			return errors.New(strings.Join(lines, "\n"))
		}

		if t.Metadata().Type == "long" {
			runType = RunTypeLong
		}

		tasks[id] = t
		taskStatus.set(id, TaskStatusNotStarted)

		for _, d := range t.Metadata().Triggers {
			byTrigger[d] = append(byTrigger[d], id)
			ingestTask(d)
		}
		for _, d := range t.Metadata().Dependencies {
			byDep[d] = append(byDep[d], id)
			if err := ingestTask(d); err != nil {
				return err
			}
		}
		for _, w := range t.Metadata().Watch {
			byWatch[w] = append(byWatch[w], id)
		}

		return nil
	}
	if err := ingestTask(taskID); err != nil {
		return nil, err
	}

	run := Run{
		mu: newMutex("run"),

		taskStatus: taskStatus,

		dir:     dir,
		runType: runType,
		rootID:  taskID,
		tasks:   tasks,

		byDep:     byDep,
		byTrigger: byTrigger,
		byWatch:   byWatch,
	}

	return &run, nil
}

// A Run represents an execution of a task, including,
//   - execution of other tasks that it depends on
//   - configuration of file-watches for retriggering tasks.
//
// A Run is safe to access concurrently from multiple goroutines.
type Run struct {
	mu *mutex

	taskStatus *safeMap[TaskStatus]

	// read-only
	runType RunType
	// read-only
	rootID string
	// read-only
	dir string
	// read-only
	tasks Tasks

	// read-only
	byDep map[string][]string
	// read-only
	byTrigger map[string][]string
	// read-only
	byWatch map[string][]string

	out MultiWriter
}

//go:generate go run golang.org/x/tools/cmd/stringer -type TaskStatus
type TaskStatus int

const (
	taskStatusInvalid TaskStatus = iota
	TaskStatusNotStarted
	TaskStatusRunning
	TaskStatusRestarting
	TaskStatusFailed
	TaskStatusDone
)

func runAsync(ctx context.Context, t Task, stdout io.Writer) *worker {
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan error)
	go func() {
		c <- t.Start(ctx, stdout)
	}()
	return &worker{c, cancel}
}

type worker struct {
	c      chan error
	cancel func()
}

func (w *worker) stop() { w.cancel() }

// MultiWriter is the interface Runs use to display UI. To start a Run, you
// must pass a MultiWriter into [Run.Start].
//
// MultiWriter is a subset of [UI], so the UIs produced by [NewTUI] and
// [NewPrinter] implement MultiWriter.
type MultiWriter interface {
	Writer(id string) io.Writer
}

// IDs returns the list of output stream names that a Run would write to. This
// includes the IDs of each Task that will be used in the run, plus the id
// "run", which the Run uses for messaging about the run itself.
func (r *Run) IDs() []string {
	var ids []string
	for id := range r.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	ids = append([]string{"run"}, ids...)
	return ids
}

// Tasks returns the Tasks that a Run would execute.
func (r *Run) Tasks() Tasks {
	return r.tasks
}

func (r *Run) TaskStatus(id string) TaskStatus {
	return r.taskStatus.get(id)
}

// Type returns the RunType of a run. It is RunTypeLong if any task is "long",
// otherwise it is RunTypeShort.
//
// If a run is RunTypeShort, it will exit once all of its tasks have succeeded.
// If a run is RunTypeLong, it will continue running until it is interrupted.
// File watches are only used if a run is RunTypeLong.
func (r *Run) Type() RunType {
	return r.runType
}

func (r *Run) printf(id string, style lipgloss.Style, f string, args ...interface{}) {
	r.out.Writer(id).Write([]byte(style.Render(fmt.Sprintf(f, args...)) + "\n"))
}

// Start starts the Run, waits for it to complete, and returns an error.
// Remember that "long" runs will never complete until canceled.
func (r *Run) Start(ctx context.Context, out MultiWriter) error {
	r.out = out

	// Start all the file watchers. Do this before starting tasks so that
	// tasks can trigger file watcher events.
	watches := map[string]func(){}
	fsevents := make(chan evFSEvent)
	for _, p := range r.watchedPaths() {
		watchP := path.Join(r.getDir(), p)
		r.printf("run", logStyle, "watching %s", watchP)
		p := p
		c, stop, err := watch(watchP)
		if err != nil {
			return err
		}
		watches[p] = stop
		go func() {
			for {
				if evs, ok := <-c; !ok {
					break
				} else {
					fsevents <- evFSEvent{path: p, evs: evs}
				}
			}
		}()
	}

	type exit struct {
		id  string
		err error
	}

	ran := newSafeMap[struct{}]()
	exits := newSafeMap[chan exit]()
	allExits := make(chan exit)
	cancels := newSafeMap[func()]()

	hasAllDeps := func(id string) bool {
		for _, dep := range r.tasks[id].Metadata().Dependencies {
			if !ran.has(dep) {
				return false
			}
		}
		return true
	}

	start := func(ctx context.Context, id string) {
		t := r.tasks[id]
		if t.Metadata().Type == "group" {
			allExits <- exit{id: id, err: nil}
			return
		}

		r.printf(id, logStyle, "starting")
		ctx, cancel := context.WithCancel(ctx)
		cancels.set(id, cancel)
		r.taskStatus.set(id, TaskStatusRunning)
		err := t.Start(ctx, out.Writer(id))
		select {
		case exits.get(id) <- exit{id: id, err: err}:
		case allExits <- exit{id: id, err: err}:
		}
		cancels.del(id)
	}

	// Start all the zero-dep tasks. When they finish, they'll trigger
	// their dependents.
	for id, t := range r.tasks {
		id, t := id, t
		if len(t.Metadata().Dependencies) > 0 {
			continue
		}
		go start(ctx, id)
	}

	// Run the loop! This is in a function just for control flow -- we can
	// easily break from it by returning.
	err := func() error {
		starts := make(chan string)
		for {
			select {
			case ev := <-fsevents:
				r.printf("run", logStyle, ev.print())
				invalidations := map[string]struct{}{}
				for _, id := range r.byWatch[ev.path] {
					invalidations[id] = struct{}{}
				}
				if len(invalidations) > 0 {
					var ids []string
					for id := range invalidations {
						ids = append(ids, id)
					}
					r.printf("run", logStyle, "invalidating {%s}", strings.Join(ids, ", "))
					go func() {
						for _, id := range ids {
							starts <- id
						}
					}()
				}
			case ev := <-allExits:
				if ev.err != nil {
					r.printf(ev.id, logStyle, "exit: %s", ev.err)
					r.taskStatus.set(ev.id, TaskStatusFailed)
				} else {
					r.taskStatus.set(ev.id, TaskStatusDone)
					ran.set(ev.id, struct{}{})
					r.printf(ev.id, logStyle, "exit ok")
				}

				if r.runType == RunTypeShort {
					// In short runs, exit when the root task does.
					if r.rootID == ev.id {
						return ev.err
					}
					// In short runs, exit when any task fails.
					if ev.err != nil {
						return ev.err
					}
				}

				t := r.tasks[ev.id]
				tm := t.Metadata()
				// If the run is "long" and the task exit was
				// unexpected, retry in 1s.
				if r.runType == RunTypeLong && ev.err != nil {
					r.printf(ev.id, logStyle, "retrying in 1 second")
					go func() {
						r.taskStatus.set(ev.id, TaskStatusRestarting)
						time.Sleep(time.Second)
						r.printf(ev.id, logStyle, "retrying")
						starts <- ev.id
					}()
					continue
				}
				// If the task is "long", retry as a keepalive
				if tm.Type == "long" {
					r.taskStatus.set(ev.id, TaskStatusRestarting)
					go func() { starts <- ev.id }()
				}

				// If the task succeeded,
				// - invalidate all tasks that list this as a trigger, and,
				// - invalidate "short" or unstarted tasks that
				//   list this as a dependency and have all of
				//   their dependencies met.
				invalidations := map[string]struct{}{}
				for _, id := range r.byTrigger[ev.id] {
					invalidations[id] = struct{}{}
				}
				for _, id := range r.byDep[ev.id] {
					isShort := r.tasks[id].Metadata().Type == "short"
					isRunning := cancels.has(id)
					isReady := hasAllDeps(id)
					if isReady && (isShort || !isRunning) {
						invalidations[id] = struct{}{}
					}
				}
				if len(invalidations) > 0 {
					var ids []string
					for id := range invalidations {
						ids = append(ids, id)
					}
					r.printf(ev.id, logStyle, "invalidating {%s}", strings.Join(ids, ", "))
					go func() {
						for _, id := range ids {
							starts <- id
						}
					}()
				}
			case <-ctx.Done():
				return nil
			case id := <-starts:
				go func() {
					if cancels.has(id) {
						cancels.get(id)()
						<-exits.get(id)
					}
					start(ctx, id)
				}()
			}
		}
	}()

	if err != nil {
		r.printf("run", errorStyle, "failed")
	} else {
		r.printf("run", logStyle, "done")
	}

	r.mu.printf("stop watches")
	for _, stop := range watches {
		stop()
	}
	r.mu.printf("stopped watches")

	r.mu.printf("stopping tasks")
	for _, k := range cancels.keys() {
		cancel := cancels.get(k)
		cancel()
	}
	r.mu.printf("stopped tasks")

	// TODO: wait for the tasks to all die
	return err
}

// A Run's RunType is RunTypeLong if any task is "long", otherwise it is
// RunTypeShort.
//
// If a run is RunTypeShort, it will exit once all of its tasks have succeeded.
// If a run is RunTypeLong, it will continue running until it is interrupted.
// File watches are only used if a run is RunTypeLong.
type RunType int

//go:generate go run golang.org/x/tools/cmd/stringer -type RunType

const (
	runTypeInvalid RunType = iota
	RunTypeShort
	RunTypeLong
)

func (r *Run) getRunType() RunType {
	return r.runType
}

func (r *Run) getDir() string {
	return r.dir
}

func (r *Run) watchedPaths() []string {
	var ps []string
	for p := range r.byWatch {
		ps = append(ps, p)
	}
	return ps
}

func (r *Run) idsByWatch(path string) []string {
	return r.byWatch[path]
}

func (r *Run) idsByTrigger(id string) []string {
	return r.byTrigger[id]
}

func (r *Run) idsByDep(id string) []string {
	return r.byDep[id]
}

func (r *Run) getTask(id string) Task {
	return r.tasks[id]
}

func (r *Run) taskMetadata(id string) TaskMetadata {
	return r.tasks[id].Metadata()
}

type evFSEvent struct {
	path string
	evs  []eventInfo
}

func (e evFSEvent) print() string {
	var b strings.Builder
	b.WriteString("watched file changes:\n")
	for _, ev := range e.evs {
		fmt.Fprintf(&b, "  %s %s\n", ev.event, ev.path)
	}
	return strings.TrimSpace(b.String())
}
