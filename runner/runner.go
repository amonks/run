package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"monks.co/run/internal/executor"
	"monks.co/run/internal/mutex"
	"monks.co/run/internal/watcher"
	"monks.co/run/task"
)

// --- Message types for the single-channel event loop ---

type (
	msgRunTask   string
	msgTaskReady string
	msgTaskExit  struct {
		id   string
		err  error
		exec *executor.Executor
	}
	msgFSEvent struct {
		path string
		evs  []watcher.EventInfo
	}
	msgInvalidate string
	msgAddTasks   []string
	msgRemoveTask string
)

// InternalTaskInterleaved is the ID used for the interleaved output stream.
const InternalTaskInterleaved = "@interleaved"

// InternalTaskWatch is the ID used for file-watcher messaging.
const InternalTaskWatch = "@watch"

// New creates an executable Run from a task library, a root task ID, and a
// display [MultiWriter].
//
// The runType parameter controls the run's lifecycle: [RunTypeShort] exits
// once the root task succeeds or any task fails; [RunTypeLong] keeps running
// until the context is canceled, restarting failed tasks with backoff.
//
// The out [MultiWriter] receives per-task output writers. Its Writer method
// is not called until [Run.Start].
func New(runType RunType, dir string, allTasks task.Library, taskID string, out MultiWriter) (*Run, error) {
	if err := allTasks.Validate(); err != nil {
		return nil, err
	}

	if !allTasks.Has(taskID) {
		lines := []string{fmt.Sprintf("Task %s not found. Tasks are,", taskID)}
		for _, id := range allTasks.IDs() {
			lines = append(lines, " - "+id)
		}
		lines = append(lines, "Run `run -list` for more information about the available tasks.")
		return nil, errors.New(strings.Join(lines, "\n"))
	}

	tasks := allTasks.Subtree(taskID)

	taskStatus := map[string]TaskStatus{}
	for _, id := range tasks.IDs() {
		taskStatus[id] = TaskStatusNotStarted
	}

	run := Run{
		mu: mutex.New("run"),

		taskStatus:      taskStatus,
		restartAttempts: map[string]int{},
		ran:             map[string]struct{}{},
		executors:       map[string]*executor.Executor{},
		writers:         map[string]io.Writer{},
		watches:         map[string]func(){},

		input: make(chan any, 256),

		out:      out,
		allTasks: allTasks,
		dir:      dir,
		runType:  runType,
		rootID:   taskID,

		tasks:          tasks,
		requestedTasks: map[string]struct{}{taskID: {}},
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

	// Mutable state, guarded by mu:
	taskStatus      map[string]TaskStatus
	restartAttempts map[string]int
	ran             map[string]struct{}
	executors       map[string]*executor.Executor
	writers         map[string]io.Writer
	watches         map[string]func() // active file watchers, keyed by path
	tasks           task.Library      // active subset of allTasks
	requestedTasks  map[string]struct{}

	// Single message channel for the event loop.
	input chan any

	// Read-only after construction:
	out      MultiWriter
	allTasks task.Library // full task universe
	runType  RunType
	rootID   string
	dir      string
}

//go:generate go run golang.org/x/tools/cmd/stringer -type TaskStatus
type TaskStatus int

const (
	taskStatusInvalid TaskStatus = iota
	TaskStatusNotStarted
	TaskStatusRunning
	TaskStatusRestarting
	TaskStatusFailed
	TaskStatusCanceled
	TaskStatusDone
)

// MultiWriter is the interface Runs use to display UI. A MultiWriter must
// be passed to [New] when creating a Run.
//
// MultiWriter is a subset of [UI], so the UIs produced by [printer.New]
// implement MultiWriter.
type MultiWriter interface {
	Writer(id string) io.Writer
}

// IDs returns the list of output stream names that a Run would write to. This
// includes the IDs of each Task that will be used in the run, plus (if
// applicable) the id "@watch", which the Run uses for messaging about file
// watchers.
func (r *Run) IDs() []string {
	defer r.mu.Lock("IDs").Unlock()
	var ids []string
	if len(r.tasks.Watches()) > 0 {
		ids = append(ids, InternalTaskWatch)
	}
	return append(ids, r.tasks.IDs()...)
}

// Tasks returns the Library that a Run would execute.
func (r *Run) Tasks() task.Library {
	defer r.mu.Lock("Tasks").Unlock()
	return r.tasks
}

// TaskStatus, given a task ID, returns that task's TaskStatus.
func (r *Run) TaskStatus(id string) TaskStatus {
	defer r.mu.Lock("TaskStatus").Unlock()
	return r.taskStatus[id]
}

// Invalidate asks a task to rerun.
func (r *Run) Invalidate(id string) {
	if !r.tasks.Has(id) {
		return
	}
	r.input <- msgInvalidate(id)
}

// Type returns the RunType passed to [New].
func (r *Run) Type() RunType {
	return r.runType
}

// Start starts the Run, waits for it to complete, and returns an error.
// Remember that "long" runs will never complete until canceled.
func (r *Run) Start(ctx context.Context) error {
	// Set up writers. Use a snapshot of IDs to avoid holding mu while
	// calling IDs() (which also takes mu).
	ids := r.IDs()
	r.mu.Lock("Start:writers")
	for _, id := range ids {
		r.writers[id] = newOutputWriter(r.out.Writer(id))
	}
	r.mu.Unlock()

	// Start all the file watchers. Do this before starting tasks so that
	// tasks can trigger file watcher events.
	for _, p := range r.tasks.Watches() {
		if err := r.startWatcher(p); err != nil {
			return err
		}
	}

	// Start all the zero-dep tasks. Snapshot task IDs to avoid holding
	// the lock while sending to the channel.
	r.mu.Lock("Start:zeroDep")
	taskIDs := r.tasks.IDs()
	r.mu.Unlock()
	for _, id := range taskIDs {
		t := r.tasks.Get(id)
		if len(t.Metadata().Dependencies) > 0 {
			continue
		}
		r.input <- msgRunTask(id)
	}

	// Run the event loop.
	var loopErr error
	for loopErr == nil {
		select {
		case msg := <-r.input:
			loopErr = r.handleMessage(ctx, msg)
		case <-ctx.Done():
			loopErr = &runExitError{err: nil}
		}
	}

	// Cleanup: stop watchers, cancel all executors.
	r.mu.Lock("Start:cleanup")
	for _, stop := range r.watches {
		stop()
	}
	for _, exec := range r.executors {
		exec.Cancel()
	}
	r.mu.Unlock()

	// Unwrap the sentinel.
	var exitErr *runExitError
	if errors.As(loopErr, &exitErr) {
		return exitErr.err
	}
	return loopErr
}

// Add dynamically adds tasks (by ID) to the active run. The task IDs must
// exist in the allTasks collection passed to New. Their transitive
// dependencies, triggers, and watches are also activated.
func (r *Run) Add(ids ...string) {
	r.input <- msgAddTasks(ids)
}

// Remove dynamically removes a task (by ID) from the active run. Tasks
// exclusively owned by the removed task are also cleaned up.
func (r *Run) Remove(id string) {
	r.input <- msgRemoveTask(id)
}

// handleMessage dispatches a message to the appropriate handler.
// It returns a non-nil error only when the run should exit.
func (r *Run) handleMessage(ctx context.Context, msg any) error {
	switch msg := msg.(type) {
	case msgRunTask:
		r.handleRunTask(ctx, string(msg))
	case msgTaskReady:
		r.handleTaskReady(string(msg))
	case msgTaskExit:
		return r.handleTaskExit(msg)
	case msgFSEvent:
		r.handleFSEvent(msg)
	case msgInvalidate:
		r.handleInvalidate(string(msg))
	case msgAddTasks:
		r.handleAddTasks(ctx, []string(msg))
	case msgRemoveTask:
		r.handleRemoveTask(string(msg))
	}
	return nil
}

// handleRunTask cancels any existing executor for the task, creates a new
// one, and starts the task.
func (r *Run) handleRunTask(ctx context.Context, id string) {
	// Cancel the old executor synchronously if one exists.
	r.mu.Lock("handleRunTask:read")
	oldExec := r.executors[id]
	r.mu.Unlock()
	if oldExec != nil {
		oldExec.Cancel()
	}

	r.printf(id, logStyle, "starting")

	t := r.tasks.Get(id)
	exec := executor.New()

	r.mu.Lock("handleRunTask:write")
	r.executors[id] = exec
	w := r.writers[id]
	if t.Metadata().Type == "short" {
		r.taskStatus[id] = TaskStatusRunning
	} else if t.Metadata().Type == "long" {
		r.taskStatus[id] = TaskStatusRestarting
	}
	r.mu.Unlock()

	// Execute the task with an onReady channel.
	onReady := make(chan struct{})
	exec.Execute(ctx, func(ctx context.Context) error {
		return t.Start(ctx, onReady, w)
	})

	// Listen for readiness signal or task exit. Status updates happen
	// in handleTaskReady (inside the event loop) to avoid racing with
	// handleTaskExit.
	go func() {
		select {
		case <-onReady:
			r.input <- msgTaskReady(id)
		case <-exec.Done():
			// Task exited before signaling readiness.
			// If it succeeded, signal readiness so dependents can start.
			if exec.Err() == nil {
				r.input <- msgTaskReady(id)
			}
		}
	}()

	// Forward the exit event.
	go func() {
		<-exec.Done()
		r.input <- msgTaskExit{id: id, err: exec.Err(), exec: exec}
	}()
}

// handleTaskReady marks a task as ran and starts eligible dependents/triggers.
func (r *Run) handleTaskReady(id string) {
	r.mu.Lock("handleTaskReady")
	r.ran[id] = struct{}{}
	// Promote Restarting → Running for long tasks that have signaled
	// readiness. Don't overwrite terminal states (Done, Failed) — a
	// late-arriving msgTaskReady must not clobber a status that
	// handleTaskExit already set.
	if r.taskStatus[id] == TaskStatusRestarting {
		r.taskStatus[id] = TaskStatusRunning
	}
	r.mu.Unlock()

	t := r.tasks.Get(id)
	tm := t.Metadata()

	invalidations := map[string]struct{}{}

	// If this task is short, invalidate all tasks that list this as a trigger.
	if tm.Type == "short" {
		for _, depID := range r.tasks.WithTrigger(id) {
			invalidations[depID] = struct{}{}
		}
	}

	// Invalidate "short" or unstarted tasks that list this as a
	// dependency and have all of their dependencies met.
	for _, depID := range r.tasks.WithDependency(id) {
		isShort := r.tasks.Get(depID).Metadata().Type == "short"
		r.mu.Lock("handleTaskReady:check")
		_, isRunning := r.executors[depID]
		r.mu.Unlock()
		isReady := r.hasAllDeps(depID)
		if isReady && (isShort || !isRunning) {
			invalidations[depID] = struct{}{}
		}
	}

	if len(invalidations) > 0 {
		var ids []string
		for depID := range invalidations {
			ids = append(ids, depID)
		}
		r.printf(id, logStyle, "invalidating {%s}", strings.Join(ids, ", "))
		for _, depID := range ids {
			r.input <- msgRunTask(depID)
		}
	}
}

// handleTaskExit processes a task exit event.
// Returns a non-nil error only when the run should exit (short-run termination).
func (r *Run) handleTaskExit(msg msgTaskExit) error {
	// Check for stale or removed executor. Discard this message if:
	// - the task has been removed (no current executor), or
	// - the current executor is different from the one that exited.
	r.mu.Lock("handleTaskExit:stale")
	currentExec := r.executors[msg.id]
	r.mu.Unlock()
	if currentExec == nil || !currentExec.Is(msg.exec) {
		return nil
	}

	if msg.err != nil {
		r.printf(msg.id, logStyle, "exit: %s", msg.err)
		r.mu.Lock("handleTaskExit:failed")
		r.taskStatus[msg.id] = TaskStatusFailed
		r.mu.Unlock()
	} else {
		r.mu.Lock("handleTaskExit:done")
		r.taskStatus[msg.id] = TaskStatusDone
		r.ran[msg.id] = struct{}{}
		r.mu.Unlock()
		r.printf(msg.id, logStyle, "exit ok")
	}

	if r.runType == RunTypeShort {
		// In short runs, exit when the root task does, or when any
		// task fails.
		if r.rootID == msg.id || msg.err != nil {
			// Even though the run is over, it's important to
			// update the task statuses. The UI might remain open,
			// and should display each task's final status.
			r.mu.Lock("handleTaskExit:shortEnd")
			for k, s := range r.taskStatus {
				switch s {
				case TaskStatusRunning, TaskStatusRestarting:
					r.taskStatus[k] = TaskStatusCanceled
				}
			}
			r.mu.Unlock()
			return &runExitError{err: msg.err}
		}
	}

	t := r.tasks.Get(msg.id)
	tm := t.Metadata()

	// If the run is "long" and the task exit was unexpected, retry
	// with exponential backoff.
	if r.runType == RunTypeLong && msg.err != nil {
		r.mu.Lock("handleTaskExit:backoff")
		r.restartAttempts[msg.id]++
		attempts := r.restartAttempts[msg.id]
		r.mu.Unlock()

		delay := min(time.Second*(1<<(attempts-1)), 30*time.Second)
		delaySec := int(delay.Seconds())
		if delaySec == 1 {
			r.printf(msg.id, logStyle, "retrying in 1 second")
		} else {
			r.printf(msg.id, logStyle, "retrying in %d seconds", delaySec)
		}
		go func() {
			r.mu.Lock("handleTaskExit:backoff:status")
			r.taskStatus[msg.id] = TaskStatusRestarting
			r.mu.Unlock()
			time.Sleep(delay)
			r.printf(msg.id, logStyle, "retrying")
			r.input <- msgRunTask(msg.id)
		}()
		return nil
	}

	// If the task is "long", retry as a keepalive.
	if tm.Type == "long" {
		r.mu.Lock("handleTaskExit:keepalive")
		r.restartAttempts[msg.id] = 0
		r.taskStatus[msg.id] = TaskStatusRestarting
		r.mu.Unlock()
		r.input <- msgRunTask(msg.id)
	}

	return nil
}

// handleFSEvent processes a file system event and restarts affected tasks.
func (r *Run) handleFSEvent(msg msgFSEvent) {
	r.printf(InternalTaskWatch, logStyle, "%s", printFSEvent(msg))

	invalidations := map[string]struct{}{}
	for _, id := range r.tasks.WithWatch(msg.path) {
		if r.hasAllDeps(id) {
			invalidations[id] = struct{}{}
		}
	}
	if len(invalidations) > 0 {
		var ids []string
		for id := range invalidations {
			ids = append(ids, id)
		}
		r.printf(InternalTaskWatch, logStyle, "invalidating {%s}", strings.Join(ids, ", "))
		for _, id := range ids {
			r.mu.Lock("handleFSEvent:resetBackoff")
			r.restartAttempts[id] = 0
			r.mu.Unlock()
			r.input <- msgRunTask(id)
		}
	}
}

// handleInvalidate resets backoff and restarts a task.
func (r *Run) handleInvalidate(id string) {
	r.mu.Lock("handleInvalidate")
	status := r.taskStatus[id]
	r.restartAttempts[id] = 0
	r.mu.Unlock()

	switch status {
	case TaskStatusRunning, TaskStatusDone, TaskStatusFailed, TaskStatusCanceled:
		r.input <- msgRunTask(id)
	}
}

// handleAddTasks activates tasks and their transitive dependencies, sets up
// watchers, creates writers, and starts zero-dep tasks.
func (r *Run) handleAddTasks(ctx context.Context, ids []string) {
	r.mu.Lock("handleAddTasks:requested")
	oldTasks := r.tasks
	for _, id := range ids {
		if r.allTasks.Has(id) {
			r.requestedTasks[id] = struct{}{}
		}
	}
	allRequested := make([]string, 0, len(r.requestedTasks))
	for id := range r.requestedTasks {
		allRequested = append(allRequested, id)
	}
	r.tasks = r.allTasks.Subtree(allRequested...)
	newTasks := r.tasks
	r.mu.Unlock()

	// Set up newly added tasks.
	var newlyAdded []string
	for _, id := range newTasks.IDs() {
		if !oldTasks.Has(id) {
			newlyAdded = append(newlyAdded, id)
		}
	}

	r.mu.Lock("handleAddTasks:setup")
	for _, id := range newlyAdded {
		r.taskStatus[id] = TaskStatusNotStarted
		if r.out != nil {
			r.writers[id] = newOutputWriter(r.out.Writer(id))
		}
	}
	r.mu.Unlock()

	// Start new file watchers for any watch paths that don't have one yet.
	r.mu.Lock("handleAddTasks:watches")
	var newWatchPaths []string
	for _, p := range newTasks.Watches() {
		if _, exists := r.watches[p]; !exists {
			newWatchPaths = append(newWatchPaths, p)
		}
	}
	r.mu.Unlock()
	for _, p := range newWatchPaths {
		r.startWatcher(p)
	}

	// Ensure the @watch writer exists if we now have watches.
	r.mu.Lock("handleAddTasks:watchWriter")
	if len(newTasks.Watches()) > 0 {
		if _, ok := r.writers[InternalTaskWatch]; !ok && r.out != nil {
			r.writers[InternalTaskWatch] = newOutputWriter(r.out.Writer(InternalTaskWatch))
		}
	}
	r.mu.Unlock()

	// Start newly-added zero-dep tasks.
	for _, id := range newlyAdded {
		t := r.allTasks.Get(id)
		if len(t.Metadata().Dependencies) == 0 {
			r.input <- msgRunTask(id)
		}
	}
}

// handleRemoveTask deactivates a task and recomputes the active subtree.
func (r *Run) handleRemoveTask(id string) {
	r.mu.Lock("handleRemoveTask:check")
	active := r.tasks.Has(id)
	r.mu.Unlock()
	if !active {
		return
	}

	r.mu.Lock("handleRemoveTask:recompute")
	oldTasks := r.tasks
	delete(r.requestedTasks, id)
	allRequested := make([]string, 0, len(r.requestedTasks))
	for rid := range r.requestedTasks {
		allRequested = append(allRequested, rid)
	}
	sub := r.allTasks.Subtree(allRequested...)
	// The removed task may still appear in the subtree as a transitive
	// dependency of another requested task. Explicitly exclude it.
	var filteredTasks []task.Task
	for _, tid := range sub.IDs() {
		if tid != id {
			filteredTasks = append(filteredTasks, sub.Get(tid))
		}
	}
	r.tasks = task.NewLibrary(filteredTasks...)
	newTasks := r.tasks
	r.mu.Unlock()

	// Determine tasks to remove (in old but not in new).
	var toRemove []string
	for _, oid := range oldTasks.IDs() {
		if !newTasks.Has(oid) {
			toRemove = append(toRemove, oid)
		}
	}

	r.mu.Lock("handleRemoveTask:cleanup")
	for _, rid := range toRemove {
		// Cancel executor.
		if exec, ok := r.executors[rid]; ok {
			go exec.Cancel()
			delete(r.executors, rid)
		}

		// Clean up status maps.
		delete(r.taskStatus, rid)
		delete(r.restartAttempts, rid)
		delete(r.ran, rid)
		delete(r.writers, rid)
	}

	// Stop watchers for paths no longer watched.
	oldWatches := map[string]struct{}{}
	for _, p := range oldTasks.Watches() {
		oldWatches[p] = struct{}{}
	}
	for p := range oldWatches {
		if !newTasks.HasWatch(p) {
			if stop, ok := r.watches[p]; ok {
				stop()
				delete(r.watches, p)
			}
		}
	}
	r.mu.Unlock()
}

// startWatcher starts a file watcher for the given path and stores it in
// r.watches.
func (r *Run) startWatcher(p string) error {
	watchP := filepath.Join(r.dir, p)
	r.printf(InternalTaskWatch, logStyle, "watching %s", watchP)
	c, stop, err := watcher.Watch(watchP)
	if err != nil {
		return err
	}
	r.mu.Lock("startWatcher")
	r.watches[p] = stop
	r.mu.Unlock()
	go func() {
		for evs := range c {
			r.input <- msgFSEvent{path: p, evs: evs}
		}
	}()
	return nil
}

// --- Helpers ---

func (r *Run) hasAllDeps(id string) bool {
	r.mu.Lock("hasAllDeps")
	defer r.mu.Unlock()
	for _, dep := range r.tasks.Get(id).Metadata().Dependencies {
		if _, ok := r.ran[dep]; !ok {
			return false
		}
	}
	return true
}

func (r *Run) printf(id string, style lipgloss.Style, f string, args ...any) {
	r.mu.Lock("printf")
	w := r.writers[id]
	r.mu.Unlock()
	if w == nil {
		return
	}
	s := fmt.Sprintf(f, args...)
	w.Write([]byte(style.Render(s) + "\n"))
}

func printFSEvent(e msgFSEvent) string {
	var b strings.Builder
	b.WriteString("watched file changes:\n")
	for _, ev := range e.evs {
		fmt.Fprintf(&b, "  %s %s\n", ev.Event, ev.Path)
	}
	return strings.TrimSpace(b.String())
}

// runExitError is a sentinel wrapper used by handleTaskExit to signal that the
// run should exit. It unwraps to nil if the underlying error is nil (clean
// exit of root task).
type runExitError struct {
	err error
}

func (e *runExitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *runExitError) Unwrap() error {
	return e.err
}

// RunType controls the run's lifecycle behavior.
//
// If a run is RunTypeShort, it will exit once the root task succeeds or any
// task fails. If a run is RunTypeLong, it will continue running until it is
// interrupted, restarting failed tasks with exponential backoff.
type RunType int

//go:generate go run golang.org/x/tools/cmd/stringer -type RunType

const (
	runTypeInvalid RunType = iota
	RunTypeShort
	RunTypeLong
)
