package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/run/internal/executor"
	"github.com/amonks/run/internal/mutex"
	"github.com/amonks/run/internal/outputwriter"
	"github.com/amonks/run/internal/styles"
	"github.com/amonks/run/internal/watcher"
	"github.com/amonks/run/tasks"
	"github.com/charmbracelet/lipgloss"
)

type Runner struct {
	mode    RunnerMode
	library tasks.Library
	dir     string
	mw      MultiWriter

	input chan any

	// Take mu to touch requestedTasks, status, or watchers.
	//
	// The event loop takes mu when handling an event, so observers are
	// guaranteed to see consistent values: never a partially-completed turn
	// of the event loop.
	mu             *mutex.Mutex
	requestedTasks map[string]struct{}
	status         map[string]TaskStatus
	ready          map[string]bool
	executors      map[string]*executor.Executor
	watchers       map[string]func()
}

func New(mode RunnerMode, lib tasks.Library, dir string, mw MultiWriter) *Runner {
	return &Runner{
		mode:    mode,
		library: lib,
		dir:     dir,
		mw:      wrapMultiWriter(mw, outputwriter.New),
		input:   make(chan any),

		mu:             mutex.New("runner"),
		requestedTasks: map[string]struct{}{},
		status:         map[string]TaskStatus{},
		ready:          map[string]bool{},
		executors:      map[string]*executor.Executor{},
		watchers:       map[string]func(){},
	}
}

// Run starts the runner and does not return until it's done.
func (r *Runner) Run(ctx context.Context, requestedTaskIDs ...string) error {

	r.mu.Printf("adding tasks to run: <%s>", strings.Join(requestedTaskIDs, ","))
	go func() { r.Add(requestedTaskIDs...) }()

	// Run the loop! This function is immediately executed; wrapping in a
	// function just gives us a nice way to top-level break (by returning).
	err := func() error {
		for {
			select {
			case <-ctx.Done():
				r.mu.Printf("run is canceled")
				r.printf(InternalTaskInterleaved, styles.Log, "run canceled")
				return context.Canceled

			case msg := <-r.input:
				r.mu.Printf("msg turn: %s", msg)

				switch msg := msg.(type) {

				case msgActivateSubtree:
					id := string(msg)
					if err := r.activateSubtree(id); err != nil {
						return err
					}

				case msgDeactivateSubtree:
					id := string(msg)
					if err := r.deactivateSubtree(id); err != nil {
						return err
					}

				case msgFSEvent:
					if err := r.handleFSEvent(msg); err != nil {
						return err
					}

				case msgRunTask:
					id := string(msg)
					if err := r.runTask(ctx, id); err != nil {
						return err
					}

				case msgTaskReady:
					id := string(msg)
					if err := r.handleTaskReady(id); err != nil {
						return err
					}

				case msgTaskExit:
					if err := r.handleTaskExit(msg.id, msg.error); err != nil {
						return err
					}

				case msgInvalidateTask:
					id := string(msg)
					if err := r.runTask(ctx, id); err != nil {
						return err
					}

				case msgRunnerDone:
					return nil
				}
			}
		}
	}()

	// Clean up.

	// Cancel all the file watchers.
	r.mu.Printf("stopping watchers")
	for p, stop := range r.watchers {
		r.printf(InternalTaskWatch, styles.Log, "stopping watcher on '%s'", p)
		stop()
	}

	// Cancel all the tasks.
	// TODO: parallel
	r.mu.Lock("cancel executors")
	for id, executor := range r.executors {
		r.printf(id, styles.Log, "canceling")
		executor.Cancel()
	}
	r.mu.Unlock()
	r.mu.Printf("done canceling")

	// Done!
	r.printf(InternalTaskInterleaved, styles.Log, "done")
	r.mu.Printf("done")

	return err
}

func (r *Runner) Add(ids ...string) {
	for _, id := range ids {
		r.input <- msgActivateSubtree(id)
	}
}

func (r *Runner) Remove(id string) {
	r.input <- msgDeactivateSubtree(id)
}

func (r *Runner) Invalidate(id string) {
	r.printf(id, styles.Log, "manually invalidated")
	r.input <- msgInvalidateTask(id)
}

func (r *Runner) Library() tasks.Library {
	return r.library
}

func (r *Runner) Status() Status {
	r.mu.Lock("Status")
	defer r.mu.Unlock()

	status := Status{
		AllTasks:       nil,
		MetaTasks:      nil,
		RequestedTasks: nil,
		ActiveTasks:    nil,
		InactiveTasks:  nil,
		TaskStatus:     make(map[string]TaskStatus, len(r.status)),
	}

	activeSubtree := r.activeSubtree()

	status.MetaTasks = append(status.MetaTasks, InternalTaskInterleaved)
	if watches := activeSubtree.Watches(); len(watches) > 0 {
		status.MetaTasks = append(status.MetaTasks, InternalTaskWatch)
	}

	for _, id := range r.library.IDs() {
		if _, isRequested := r.requestedTasks[id]; isRequested {
			status.RequestedTasks = append(status.RequestedTasks, id)
		} else if activeSubtree.Has(id) {
			status.ActiveTasks = append(status.ActiveTasks, id)
		} else {
			status.InactiveTasks = append(status.InactiveTasks, id)
		}
		status.TaskStatus[id] = r.status[id]
	}

	for _, id := range status.MetaTasks {
		status.AllTasks = append(status.AllTasks, id)
	}
	for _, id := range status.RequestedTasks {
		status.AllTasks = append(status.AllTasks, id)
	}
	for _, id := range status.ActiveTasks {
		status.AllTasks = append(status.AllTasks, id)
	}
	for _, id := range status.InactiveTasks {
		status.AllTasks = append(status.AllTasks, id)
	}

	return status
}

func (r *Runner) printf(id string, style lipgloss.Style, f string, args ...any) {
	w := r.mw.Writer(id)
	s := fmt.Sprintf(f, args...)
	w.Write([]byte(style.Render(s) + "\n"))
}

func (r *Runner) activeSubtree() tasks.Library {
	var ids []string
	for id := range r.requestedTasks {
		ids = append(ids, id)
	}
	return r.library.Subtree(ids...)
}

func (r *Runner) activateSubtree(id string) error {
	r.mu.Lock("activateSubtree")
	defer r.mu.Unlock()

	// Stop the whole run if the requested task isn't in the library.
	if !r.library.Has(id) {
		lines := []string{fmt.Sprintf("Task %s not found. Tasks are,", id)}
		for _, id := range r.library.IDs() {
			lines = append(lines, " - "+id)
		}
		lines = append(lines, "Run `run -list` for more information about the available tasks.")
		return errors.New(strings.Join(lines, "\n"))
	}

	// Add the new task to the requested tasks set. Do nothing if it's
	// already in there.
	if _, alreadyStarted := r.requestedTasks[id]; alreadyStarted {
		return nil
	} else {
		r.requestedTasks[id] = struct{}{}
	}

	// Find the task's dependencies.
	subtree := r.library.Subtree(id)

	// Add the task and its dependencies to the task status set.
	var newTasks []string
	for _, id := range subtree.IDs() {
		if _, alreadyStarted := r.status[id]; !alreadyStarted {
			r.status[id] = TaskStatusNotStarted
			newTasks = append(newTasks, id)
		}
	}

	// If this is a keepalive run, first start any file watchers, so that
	// we're sure to pick up any fsevents that the tasks fire.
	if r.mode == RunnerModeKeepalive {
		for _, path := range subtree.Watches() {
			path := path
			r.printf(InternalTaskWatch, styles.Log, "watching "+path)
			c, stop, err := watcher.Watch(filepath.Join(r.dir, path))
			if err != nil {
				return fmt.Errorf("file watch error: %s", err)
			}
			r.watchers[path] = stop
			r.mu.Printf("notifying fsevent on path '%s'", path)
			go func() {
				for evs := range c {
					r.input <- msgFSEvent{path: path, evs: evs}
				}
			}()
		}
	}

	// Now start any new tasks whose dependencies are already met (eg
	// zero-dependency tasks, or tasks whose dependencies were covered by a
	// previous call to activateSubtree). When they complete, they'll
	// trigger new tasks that depend on them.
	for _, id := range newTasks {
		task := r.library.Task(id)
		isReady := true
		for _, dep := range task.Metadata().Dependencies {
			if !r.ready[dep] {
				isReady = false
				break
			}
		}
		if isReady {
			r.mu.Printf("jumpstarting '%s' because its dependencies are already met", id)
			go func() { r.input <- msgRunTask(id) }()
		}
	}

	return nil
}

func (r *Runner) deactivateSubtree(id string) error {
	r.mu.Lock("deactivateSubtree")
	defer r.mu.Unlock()

	// Error if the given root isn't in the requested task set.
	if _, isActive := r.requestedTasks[id]; !isActive {
		var tasks []string
		for t := range r.requestedTasks {
			tasks = append(tasks, t)
		}
		return fmt.Errorf("deactivateSubtree of '%s' failed because it is not active. Active tasks are %s.", id, strings.Join(tasks, ","))
	}

	// Remove the task from the requested task set.
	delete(r.requestedTasks, id)

	var remainingRoots []string
	for id := range r.requestedTasks {
		remainingRoots = append(remainingRoots, id)
	}

	// Define two subtrees: the new one and the removed one. Anything that
	// appears in the removed one but not the new one should be stopped.
	removedSubtree := r.library.Subtree(id)
	newSubtree := r.library.Subtree(remainingRoots...)

	// Cancel tasks that should be stopped.
	{
		var toCancel []string
		for _, id := range removedSubtree.IDs() {
			if !newSubtree.Has(id) {
				toCancel = append(toCancel, id)
			}
		}
		for _, id := range toCancel {
			if xtr, hasXtr := r.executors[id]; hasXtr {
				r.mu.Printf("cancel %s", id)
				xtr.Cancel()
			}
		}
	}

	// Cancel file watchers that should be stopped.
	{
		var toCancel []string
		for _, path := range removedSubtree.Watches() {
			if !newSubtree.HasWatch(path) {
				toCancel = append(toCancel, path)
			}
		}
		for _, path := range toCancel {
			if stop, hasStop := r.watchers[path]; hasStop {
				stop()
			}
		}
	}

	return nil
}

func (r *Runner) runTask(ctx context.Context, id string) error {
	r.mu.Lock("runTask")
	defer r.mu.Unlock()

	// Short circuit if the task's dependencies are not met.
	t := r.library.Task(id)
	for _, dep := range t.Metadata().Dependencies {
		if !r.ready[dep] {
			r.mu.Printf("not running %s; %s is not ready", id, dep)
			return nil
		}
	}

	// If the task is already running, cancel it before continuing.
	xtr, hasXtr := r.executors[id]
	if hasXtr && !xtr.IsDone() {
		r.mu.Printf("canceling %s before re-running it", id)
		go func() { xtr.Cancel(); r.input <- msgRunTask(id) }()
		return nil
	}

	// Actually run the task now.
	//

	r.printf(id, styles.Log, "starting")

	// Mark that the task is running.
	if t.Metadata().Type == "short" {
		r.status[id] = TaskStatusRunning
	} else if t.Metadata().Type == "long" {
		r.status[id] = TaskStatusRestarting
	} else {
		panic(fmt.Errorf("invalid type '%s' for task '%s'", t.Metadata().Type, id))
	}

	// Create an executor for the task.
	onReady := make(chan struct{})
	xtr = executor.New(func(ctx context.Context) error {
		return t.Start(ctx, onReady, r.mw.Writer(id))
	})
	r.executors[id] = xtr

	// Handle when the task becomes ready.
	stoppedEarly := make(chan struct{})
	go func() {
		select {
		case _, ok := <-onReady:
			if ok {
				r.printf(id, styles.Log, "try to send readiness signal...")
				r.input <- msgTaskReady(id)
				r.printf(id, styles.Log, "sent readiness signal.")
			}
		case <-stoppedEarly:
		}
	}()

	// Run the task in its own thread so that the event loop can continue.
	go func() {
		// Run the task.
		r.mu.Printf("starting task '%s'", id)
		xtr.Execute()
		err, notCanceled := <-xtr.Wait()

		// Done.

		// If this task has already been replaced, do nothing.
		r.mu.Lock("destroy executor")
		if !r.executors[id].Is(xtr) {
			r.mu.Unlock()
			return
		}
		// Destroy the executor.
		delete(r.executors, id)
		r.mu.Unlock()

		// If we haven't sent a readiness signal yet, cancel it.
		select {
		case stoppedEarly <- struct{}{}:
			r.printf(id, styles.Log, "canceled readiness signal")
		default:
			r.printf(id, styles.Log, "readiness signal already fired")
		}

		// If we were canceled, the canceler's goroutine was already
		// notified by the executor and resumed control flow. Otherwise,
		// back to the main loop.
		if notCanceled {
			r.printf(id, styles.Log, "send exit signal")
			r.input <- msgTaskExit{id, err}
		}
	}()

	r.mu.Printf("started")

	return nil
}

func (r *Runner) handleTaskReady(id string) error {
	r.mu.Lock("handleTaskReady")
	defer r.mu.Unlock()

	r.printf(id, styles.Log, "ready")
	r.mu.Printf("'%s' becomes ready", id)
	r.ready[id] = true
	active := r.activeSubtree()
	switch r.status[id] {
	case TaskStatusRestarting:
		r.status[id] = TaskStatusRunning
	}

	// Start tasks that depend on this one.
	for _, other := range active.WithDependency(id) {
		if r.status[other] == TaskStatusNotStarted {
			r.printf(other, styles.Log, "invalidated because it is %s and '%s' is ready", r.status[other].String(), id)
			go func() { r.input <- msgInvalidateTask(other) }()
		}
	}

	// Start tasks which are triggered by this one.
	for _, other := range active.WithTrigger(id) {
		r.printf(other, styles.Log, "invalidated because '%s' is ready", id)
		go func() { r.input <- msgInvalidateTask(other) }()
	}

	return nil
}

func (r *Runner) handleTaskExit(id string, err error) error {
	r.mu.Lock("handleTaskExit")
	defer r.mu.Unlock()

	// NOTE: This is NOT called if the task was canceled.

	// NOTE: This can be called for a task even if handleTaskReady was not
	// called, in both success and failure cases.

	// Destroy the executor.
	delete(r.executors, id)

	// Update the UI.
	if err != nil {
		r.printf(id, styles.Log, "exit: %s", err)
		r.status[id] = TaskStatusFailed
	} else {
		r.printf(id, styles.Log, "exit ok")
		r.status[id] = TaskStatusDone
	}

	// If the task succeeded, mark it as ready. Tasks that depend on it may
	// run now. We actually run them a bit below here when we dispatch
	// msgTaskReady: it's good to hold off on that until -after- we hit the
	// early-return paths, so that we don't needlessly start tasks which we
	// would immediately cancel.
	if err == nil && !r.ready[id] {
		r.mu.Printf("'%s' becomes ready 2", id)
		r.ready[id] = true
	}

	// If the runner mode is "exit", we have two exit conditions to handle.
	if r.mode == RunnerModeExit {

		// Exit when any task fails.
		if err != nil {
			return err
		}

		// Exit when all requested tasks have succeeded.
		allReady := true
		for id := range r.requestedTasks {
			if !r.ready[id] {
				allReady = false
				break
			}
		}
		if allReady {
			go func() { r.input <- msgRunnerDone{} }()
			return nil
		}
	}

	// If the runner mode is "keepalive"...
	if r.mode == RunnerModeKeepalive {
		task := r.library.Task(id)
		switch true {

		// If the task exit was unexpected, retry the task in 1s.
		case err != nil:
			go func() { time.Sleep(time.Second); r.input <- msgRunTask(id) }()

		// If the task is "long", retry the task immediately (as a
		// keepalive).
		case task.Metadata().Type == "long":
			go func() { r.input <- msgRunTask(id) }()
		}
	}

	// If the task succeeded, dispatch msgTaskReady to invalidate tasks that
	// depend on it.
	if err == nil {
		r.mu.Printf("marking '%s' as ready because it succeeded", id)
		go func() { r.input <- msgTaskReady(id) }()
	}

	return nil
}

func (r *Runner) handleFSEvent(ev msgFSEvent) error {
	r.mu.Lock("handleFSEvent")
	defer r.mu.Unlock()

	activeSubtree := r.activeSubtree()
	invalidations := activeSubtree.WithWatch(ev.path)

	// An fsevent which triggers no invalidations indicates a bug in the
	// runner, and should cause failure.
	if len(invalidations) == 0 {
		return fmt.Errorf("no invalidations from watch on '%s'", ev.path)
	}

	r.printf(InternalTaskWatch, styles.Log, "invalidating {%s}", strings.Join(invalidations, ", "))
	go func() {
		for _, id := range invalidations {
			r.input <- msgInvalidateTask(id)
		}
	}()

	return nil
}

type Status struct {
	AllTasks       []string
	MetaTasks      []string
	RequestedTasks []string
	ActiveTasks    []string
	InactiveTasks  []string
	TaskStatus     map[string]TaskStatus
}

const (
	InternalTaskInterleaved = "@interleaved"
	InternalTaskWatch       = "@watch"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type RunnerMode
type RunnerMode int

const (
	runnerModeInvalid RunnerMode = iota
	RunnerModeKeepalive
	RunnerModeExit
)

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

//go:generate go run github.com/amonks/run/cmd/messagestringer -file $GOFILE -prefix msg
type (
	msgActivateSubtree   string
	msgDeactivateSubtree string
	msgRunTask           string
	msgTaskReady         string
	msgInvalidateTask    string
	msgRunnerDone        struct{}
	msgTaskExit          struct {
		id    string
		error error
	}
	msgFSEvent struct {
		path string
		evs  []watcher.EventInfo
	}
)
