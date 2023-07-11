package run

import (
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
	states := map[string]taskState{}
	ran := map[string]bool{}
	counters := map[string]int{}
	byDep := map[string][]string{}
	byTrigger := map[string][]string{}
	byWatch := map[string][]string{}

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
		states[id] = taskStateNotStarted

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

		dir:      dir,
		runType:  runType,
		rootID:   taskID,
		tasks:    tasks,
		ran:      ran,
		states:   states,
		counters: counters,
		watches:  map[string]func(){},

		byDep:     byDep,
		byTrigger: byTrigger,
		byWatch:   byWatch,

		events: make(chan event),
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

	runType  RunType
	rootID   string
	dir      string
	tasks    Tasks
	states   map[string]taskState
	ran      map[string]bool
	counters map[string]int
	watches  map[string]func()

	byDep     map[string][]string
	byTrigger map[string][]string
	byWatch   map[string][]string

	events chan event

	out MultiWriter

	waiters []chan<- error
	stopped bool
}

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
	defer r.mu.Lock("IDs").Unlock()

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
	defer r.mu.Lock("tasks").Unlock()

	return r.tasks
}

// Type returns the RunType of a run. It is RunTypeLong if any task is "long",
// otherwise it is RunTypeShort.
//
// If a run is RunTypeShort, it will exit once all of its tasks have succeeded.
// If a run is RunTypeLong, it will continue running until it is interrupted.
// File watches are only used if a run is RunTypeLong.
func (r *Run) Type() RunType {
	defer r.mu.Lock("Type").Unlock()

	return r.runType
}

func (r *Run) printf(id string, style lipgloss.Style, f string, args ...interface{}) {
	r.out.Writer(id).Write([]byte(style.Render(fmt.Sprintf(f, args...)) + "\n"))
}

// Start starts the Run. If it returns nil, the Run is started successfully.
// After starting the run, you can wait for it to end with [Run.Wait], or stop
// it immediately with [Run.Stop].
func (r *Run) Start(out MultiWriter) error {
	defer r.mu.Lock("Start").Unlock()

	r.out = out

	// Start the event loop. Do this before anything else so that other
	// things can dispatch events.
	events := r.events
	go func() {
		for {
			ev, ok := <-events
			if !ok {
				return
			}
			r.handleEvent(ev)
		}
	}()

	// Start all the file watchers. Do this before starting tasks so that
	// tasks can trigger file watcher events.
	for p := range r.byWatch {
		watchP := path.Join(r.dir, p)
		r.printf("run", logStyle, "watching %s", watchP)
		p := p
		c, stop, err := watch(watchP)
		if err != nil {
			return err
		}
		r.watches[p] = stop
		go func() {
			for {
				evs, ok := <-c
				if !ok {
					break
				}

				r.send(evFSEvent{
					path: p,
					evs:  evs,
				})
			}
		}()
	}

	// Start all the zero-dep tasks. When they finish, they'll trigger
	// their dependents.
	for id, t := range r.tasks {
		if len(t.Metadata().Dependencies) > 0 {
			continue
		}
		go r.send(evInvalidateTask{id})
	}

	return nil
}

// Wait returns a channel that will emit one error when the Run exits, then
// close. It is ok to call Wait before calling [Run.Start]. If Wait is called
// after a Run exits, it will return a closed channel. If Wait is called more
// than once, it will return different channels, and all of the channels will
// emit when the Run exits.
func (r *Run) Wait() <-chan error {
	defer r.mu.Lock("Wait").Unlock()

	if r.stopped {
		c := make(chan error)
		close(c)
		return c
	}

	c := make(chan error)
	r.waiters = append(r.waiters, c)
	return c
}

// Stop stops a Run, including all of its tasks and watches, and returns when
// the Run has stopped. If any waiting channels were created with [Run.Wait],
// they will emit before Stop returns.
//
// It is safe (but useless) to call Stop without previously calling
// [Run.Start].
func (r *Run) Stop() {
	r.stop(nil)
}

func (r *Run) stop(err error) {
	defer r.mu.Lock("stop").Unlock()

	if r.stopped {
		r.mu.printf("[run] already stopped\n")
		return
	}
	r.stopped = true
	r.mu.printf("[run] stop watches\n")
	for _, stop := range r.watches {
		stop()
	}
	r.mu.printf("[run] stopped watches\n")

	r.mu.printf("[run] stopping tasks\n")
	for id, t := range r.tasks {
		r.states[id] = taskStateStopping
		t.Stop()
	}
	r.mu.printf("[run] stopped tasks\n")

	close(r.events)

	for _, w := range r.waiters {
		select {
		case w <- err:
		default:
		}
		close(w)
	}
}

//go:generate go run golang.org/x/tools/cmd/stringer -type taskState
type taskState int

const (
	taskStateNotStarted taskState = iota
	taskStateRunning
	taskStateRestarting
	taskStateStopping
	taskStateStopped
)

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

func (r *Run) handleEvent(ev event) {
	defer r.mu.Lock("handleEvent:" + ev.eventType()).Unlock()

	switch ev := ev.(type) {
	case evFatal:
		r.printf("run", logStyle, "fatal")
		r.stop(ev.err)
		return

	case evTaskReady:
		taskIDs := r.byDep[ev.task]
		if len(taskIDs) > 0 {
			r.printf(ev.task, logStyle, "ready, invalidating {%s}", strings.Join(taskIDs, ", "))
			for _, id := range taskIDs {
				id := id
				go r.send(evInvalidateTask{id})
			}
		}

	case evTaskExit:
		if r.states[ev.task] == taskStateStopping {
			r.printf(ev.task, logStyle, "stopped")
			r.states[ev.task] = taskStateStopped
			return
		}
		if r.states[ev.task] == taskStateRestarting {
			return
		}

		if ev.err == nil {
			r.printf(ev.task, logStyle, "exit ok")
		} else {
			r.printf(ev.task, errorStyle, "exit: %s", ev.err.Error())
		}

		r.ran[ev.task] = true

		// restart if
		// - task is long (to keepalive), or,
		// - run is long and exit was failure (to retry)
		t := r.tasks[ev.task]
		if t.Metadata().Type == "long" || (r.runType == RunTypeLong && ev.err != nil) {
			r.states[ev.task] = taskStateRestarting
			go func() {
				r.printf(ev.task, logStyle, "retrying in 1 second")
				time.Sleep(1 * time.Second)
				r.printf(ev.task, logStyle, "retrying")
				go r.send(evInvalidateTask{ev.task})
			}()
			return
		}

		r.states[ev.task] = taskStateStopped

		// If exit was unexpected and this was a short run, we're done now.
		if r.runType == RunTypeShort && ev.err != nil {
			r.printf("run", logStyle, "failed")
			go r.stop(ev.err)
			return
		}

		// if exit was exepected success, it should
		// - invalidate all tasks that list this as a trigger
		// - invalidate short tasks that list this as a dependency
		if t.Metadata().Type == "short" && ev.err == nil {
			setToInvalidate := map[string]struct{}{}
			for _, id := range r.byTrigger[ev.task] {
				setToInvalidate[id] = struct{}{}
			}
			for _, id := range r.byDep[ev.task] {
				if r.tasks[id].Metadata().Type == "short" || r.states[id] == taskStateNotStarted {
					setToInvalidate[id] = struct{}{}
				}
			}
			if len(setToInvalidate) > 0 {
				var tasksToInvalidate []string
				for id := range setToInvalidate {
					tasksToInvalidate = append(tasksToInvalidate, id)
				}
				r.printf(ev.task, logStyle, "invalidating {%s}", strings.Join(tasksToInvalidate, ", "))
				go func() {
					for id := range setToInvalidate {
						go r.send(evInvalidateTask{id})
					}
				}()
				return
			}
		}

		// If this is a short run, check if we are done now
		if r.runType == RunTypeShort {
			allStopped := true
			for _, s := range r.states {
				if s != taskStateStopped {
					allStopped = false
					break
				}
			}
			if allStopped {
				r.printf("run", logStyle, "done")
				go r.stop(ev.err)
				return
			}
		}

	case evFSEvent:
		var evs []string
		for _, ev := range ev.evs {
			evs = append(evs, ev.event+":"+ev.path)
		}
		r.printf("run", logStyle, "watched file change: {%s}", strings.Join(evs, ", "))
		taskIDs := r.byWatch[ev.path]
		if len(taskIDs) > 0 {
			r.printf("run", logStyle, "invalidating {%s}", strings.Join(taskIDs, ", "))
			for _, id := range taskIDs {
				id := id
				go r.send(evInvalidateTask{id})
			}
		}

	case evInvalidateTask:
		t := r.tasks[ev.task]
		for _, dep := range t.Metadata().Dependencies {
			if !r.ran[dep] {
				return
			}
		}

		r.printf(ev.task, logStyle, "starting")
		if r.states[ev.task] == taskStateRunning {
			r.states[ev.task] = taskStateRestarting
		}
		r.counters[ev.task] += 1
		counter := r.counters[ev.task]

		go func() {
			if err := t.Start(r.out.Writer(ev.task)); err != nil {
				r.send(evFatal{err})
				return
			}

			r.mu.Lock("set-to-running")
			if r.counters[ev.task] != counter {
				return
			}
			r.states[ev.task] = taskStateRunning
			r.mu.Unlock()

			if t.Metadata().Type == "long" {
				go func() {
					time.Sleep(50 * time.Millisecond)
					defer r.mu.Lock("ready").Unlock()
					if r.counters[ev.task] != counter {
						return
					}
					r.ran[ev.task] = true
					if r.states[ev.task] == taskStateRunning {
						go r.send(evTaskReady{ev.task})
					}
				}()
			}
			err := <-t.Wait()

			r.mu.Lock("done-waiting")
			if r.counters[ev.task] != counter {
				r.mu.Unlock()
				return
			}
			if r.states[ev.task] == taskStateRunning {
				r.mu.Unlock()
				r.send(evTaskExit{ev.task, err})
			} else {
				r.mu.Unlock()
			}
		}()

	default:
		fmt.Printf("unexpected event type: %+v\n", ev)
	}
}

func (r *Run) send(ev event) {
	r.mu.Lock("send")
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	r.events <- ev
}

type event interface {
	eventType() string
}

type evFatal struct {
	err error
}

func (e evFatal) eventType() string { return "evFatal" }

type evFSEvent struct {
	path string
	evs  []eventInfo
}

func (e evFSEvent) eventType() string { return "evFSEvent" }

type evTaskReady struct {
	task string
}

func (e evTaskReady) eventType() string { return "evTaskReady" }

type evTaskExit struct {
	task string
	err  error
}

func (e evTaskExit) eventType() string { return "evTaskExit" }

type evInvalidateTask struct {
	task string
}

func (e evInvalidateTask) eventType() string { return "evInvalidateTask" }
