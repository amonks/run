package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/run/internal/mutex"
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

	var (
		ids       = []string{}
		tasks     = map[string]Task{}
		byDep     = map[string][]string{}
		byTrigger = map[string][]string{}
		byWatch   = map[string][]string{}
	)

	// NOTE: We don't ever switch on this in a meaningful way, it's just
	// for the UI. It's always safe to set taskStatus to whatever to meet a
	// UI goal. It will never impact control flow.
	taskStatus := newSafeMap[TaskStatus]()

	var ingestTask func(string) error
	ingestTask = func(id string) error {
		if !allTasks.Has(id) {
			lines := []string{fmt.Sprintf("Task %s not found. Tasks are,", id)}
			for _, id := range allTasks.IDs() {
				lines = append(lines, " - "+id)
			}
			lines = append(lines, "Run `run -list` for more information about the available tasks.")
			return errors.New(strings.Join(lines, "\n"))
		}

		ids = append(ids, id)
		t := allTasks.Get(id)
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

	runType := RunTypeShort
	if tasks[taskID].Metadata().Type == "long" {
		runType = RunTypeLong
	}

	run := Run{
		mu: mutex.New("run"),

		taskStatus: taskStatus,

		starts: make(chan string),

		dir:     dir,
		runType: runType,
		rootID:  taskID,

		tasks: Tasks{
			ids:   ids,
			tasks: tasks,
		},

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
	mu *mutex.Mutex

	taskStatus *safeMap[TaskStatus]

	starts chan string

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
	return append([]string{"run"}, r.tasks.IDs()...)
}

// Tasks returns the Tasks that a Run would execute.
func (r *Run) Tasks() Tasks {
	return r.tasks
}

// TaskStatus, given a task ID, returns that task's TaskStatus.
func (r *Run) TaskStatus(id string) TaskStatus {
	return r.taskStatus.get(id)
}

// Invalidate asks a task to rerun. It will block until the Run gets the
// message (which is BEFORE the task is restarted).
func (r *Run) Invalidate(id string) {
	if !r.tasks.Has(id) {
		return
	}
	switch r.TaskStatus(id) {
	case TaskStatusRunning, TaskStatusDone, TaskStatusFailed:
		r.starts <- id
	default:
	}
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

// Start starts the Run, waits for it to complete, and returns an error.
// Remember that "long" runs will never complete until canceled.
func (r *Run) Start(ctx context.Context, out MultiWriter) error {
	writers := newSafeMap[io.Writer]()
	for _, id := range r.IDs() {
		writers.set(id, newOutputWriter(out.Writer(id)))
	}

	printf := func(id string, style lipgloss.Style, f string, args ...interface{}) {
		w := writers.get(id)
		s := fmt.Sprintf(f, args...)
		w.Write([]byte(style.Render(s) + "\n"))
	}

	// Start all the file watchers. Do this before starting tasks so that
	// tasks can trigger file watcher events.
	watches := map[string]func(){}
	var watcher watcher
	fsevents := make(chan evFSEvent)
	for _, p := range r.watchedPaths() {
		watchP := filepath.Join(r.getDir(), p)
		printf("run", logStyle, "watching %s", watchP)
		p := p
		c, stop, err := watcher.watch(watchP)
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
	readies := make(chan string)

	hasAllDeps := func(id string) bool {
		for _, dep := range r.tasks.Get(id).Metadata().Dependencies {
			if !ran.has(dep) {
				return false
			}
		}
		return true
	}

	start := func(ctx context.Context, id string) {
		printf(id, logStyle, "starting")

		t := r.tasks.Get(id)

		// Mark that the task is running.
		ctx, cancel := context.WithCancel(ctx)
		cancels.set(id, cancel)
		exits.set(id, make(chan exit))
		if t.Metadata().Type == "short" {
			r.taskStatus.set(id, TaskStatusRunning)
		} else if t.Metadata().Type == "long" {
			r.taskStatus.set(id, TaskStatusRestarting)
		}

		// If the task is long, send a "ready" 500ms after it starts,
		// in order to trigger tasks that depend on it.
		//
		// We don't _really_ know that it's ready -- 500ms is just a
		// heuristic. It might be nice to add something to the task
		// interface so that long tasks can tell us when they are truly
		// ready.
		stoppedEarly := make(chan struct{})
		if t.Metadata().Type == "long" {
			go func() {
				select {
				case <-stoppedEarly:
				case <-time.After(500 * time.Millisecond):
					r.taskStatus.set(id, TaskStatusRunning)
					readies <- id
				}
			}()
		}

		// Run the task.
		w := writers.get(id)
		err := t.Start(ctx, w)

		// Done. Destroy its canceler.
		cancels.del(id)

		// If we haven't sent a readiness signal yet, cancel it.
		select {
		case stoppedEarly <- struct{}{}:
		default:
		}

		// Notify that this task is done. Prefer to send to this task's
		// specific channel, but if that's not available the allTasks
		// channel is fine. Block until sending to one or the other.
		select {
		case exits.get(id) <- exit{id: id, err: err}:
			return
		default:
		}
		select {
		case allExits <- exit{id: id, err: err}:
		case exits.get(id) <- exit{id: id, err: err}:
		}
	}

	// Start all the zero-dep tasks. When they finish, they'll trigger
	// their dependents.
	for _, id := range r.tasks.IDs() {
		id, t := id, r.tasks.Get(id)
		if len(t.Metadata().Dependencies) > 0 {
			continue
		}
		go start(ctx, id)
	}

	// Run the loop! This is in a function just for control flow -- we can
	// easily break from it by returning.
	err := func() error {
		for {
			select {
			case ev := <-fsevents:
				printf("run", logStyle, ev.print())
				invalidations := map[string]struct{}{}
				for _, id := range r.byWatch[ev.path] {
					invalidations[id] = struct{}{}
				}
				if len(invalidations) > 0 {
					var ids []string
					for id := range invalidations {
						ids = append(ids, id)
					}
					printf("run", logStyle, "invalidating {%s}", strings.Join(ids, ", "))
					go func() {
						for _, id := range ids {
							r.starts <- id
						}
					}()
				}

			case id := <-r.starts:
				go func() {
					if cancels.has(id) {
						cancel, exit := cancels.get(id), exits.get(id)
						cancel()
						<-exit
					}
					start(ctx, id)
				}()

			case id := <-readies:
				t := r.tasks.Get(id)
				tm := t.Metadata()

				// Mark this task as "ran", so tasks that
				// depend on it become eligible to run.
				ran.set(id, struct{}{})

				// Invalidate tasks that depend on this one.
				invalidations := map[string]struct{}{}

				// If this task is short, invalidate all tasks
				// that list this as a trigger.
				if tm.Type == "short" {
					for _, id := range r.byTrigger[id] {
						invalidations[id] = struct{}{}
					}
				}

				// Invalidate "short" or unstarted tasks that
				// list this as a dependency and have all of
				// their dependencies met.
				for _, id := range r.byDep[id] {
					isShort := r.tasks.Get(id).Metadata().Type == "short"
					isRunning := cancels.has(id)
					isReady := hasAllDeps(id)
					if isReady && (isShort || !isRunning) {
						invalidations[id] = struct{}{}
					}
				}

				// Send the invalidations.
				if len(invalidations) > 0 {
					var ids []string
					for id := range invalidations {
						ids = append(ids, id)
					}
					printf(id, logStyle, "invalidating {%s}", strings.Join(ids, ", "))
					go func() {
						for _, id := range ids {
							r.starts <- id
						}
					}()
				}

			case ev := <-allExits:
				if ev.err != nil {
					printf(ev.id, logStyle, "exit: %s", ev.err)
					r.taskStatus.set(ev.id, TaskStatusFailed)
				} else {
					r.taskStatus.set(ev.id, TaskStatusDone)
					ran.set(ev.id, struct{}{})
					printf(ev.id, logStyle, "exit ok")
				}

				if r.runType == RunTypeShort {
					// In short runs, exit when the root
					// task does, or when any task fails.
					if r.rootID == ev.id || ev.err != nil {
						// Even though the run is over,
						// it's important to update the
						// task statuses. The UI might
						// remain open, and should
						// display each task's final
						// status.
						for _, k := range r.taskStatus.keys() {
							switch r.taskStatus.get(k) {
							case TaskStatusRunning, TaskStatusRestarting:
								r.taskStatus.set(k, TaskStatusFailed)
							}
						}
						return ev.err
					}
				}

				t := r.tasks.Get(ev.id)
				tm := t.Metadata()
				// If the run is "long" and the task exit was
				// unexpected, retry in 1s.
				if r.runType == RunTypeLong && ev.err != nil {
					printf(ev.id, logStyle, "retrying in 1 second")
					go func() {
						r.taskStatus.set(ev.id, TaskStatusRestarting)
						time.Sleep(time.Second)
						printf(ev.id, logStyle, "retrying")
						r.starts <- ev.id
					}()
					continue
				}
				// If the task is "long", retry as a keepalive
				if tm.Type == "long" {
					r.taskStatus.set(ev.id, TaskStatusRestarting)
					go func() { r.starts <- ev.id }()
				}

				// If the task succeeded, invalidate tasks that
				// depend on it
				go func() { readies <- ev.id }()

			case <-ctx.Done():
				printf("run", logStyle, "run canceled")
				return nil
			}
		}
	}()

	if err != nil {
		printf("run", errorStyle, "failed")
	} else {
		printf("run", logStyle, "done")
	}

	for _, stop := range watches {
		stop()
	}

	for _, k := range cancels.keys() {
		if !cancels.has(k) {
			continue
		}
		cancel := cancels.get(k)
		cancel()
		<-exits.get(k)
	}

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
