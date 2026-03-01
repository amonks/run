package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/run/internal/executor"
	"github.com/amonks/run/internal/mutex"
	"github.com/amonks/run/internal/watcher"
	"charm.land/lipgloss/v2"
)

// --- Message types for the single-channel event loop ---

type (
	msgRunTask    string
	msgTaskReady  string
	msgTaskExit   struct {
		id   string
		err  error
		exec *executor.Executor
	}
	msgFSEvent struct {
		path string
		evs  []watcher.EventInfo
	}
	msgInvalidate  string
	msgAddTasks    []string
	msgRemoveTask  string
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
		tasks     = map[string]Task{}
		byDep     = map[string][]string{}
		byTrigger = map[string][]string{}
		byWatch   = map[string][]string{}
	)

	taskStatus := map[string]TaskStatus{}

	// NOTE:
	// ingestTask can be called multiple times for each task.
	var ingestTask func(string) error
	ingestTask = func(id string) error {
		if _, alreadyIngested := tasks[id]; alreadyIngested {
			return nil
		}

		if !allTasks.Has(id) {
			lines := []string{fmt.Sprintf("Task %s not found. Tasks are,", id)}
			for _, id := range allTasks.IDs() {
				lines = append(lines, " - "+id)
			}
			lines = append(lines, "Run `run -list` for more information about the available tasks.")
			return errors.New(strings.Join(lines, "\n"))
		}

		t := allTasks.Get(id)
		tasks[id] = t
		taskStatus[id] = TaskStatusNotStarted

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

	// Now that we know which tasks we need, put their IDs in the same
	// order they appear in allTasks.
	ids := []string{}
	for _, id := range allTasks.IDs() {
		if _, isIncluded := tasks[id]; isIncluded {
			ids = append(ids, id)
		}
	}

	runType := RunTypeShort
	if tasks[taskID].Metadata().Type == "long" {
		runType = RunTypeLong
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

		allTasks: allTasks,
		dir:      dir,
		runType:  runType,
		rootID:   taskID,

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

	// Mutable state, guarded by mu:
	taskStatus      map[string]TaskStatus
	restartAttempts map[string]int
	ran             map[string]struct{}
	executors       map[string]*executor.Executor
	writers         map[string]io.Writer
	watches         map[string]func() // active file watchers, keyed by path
	tasks           Tasks              // active subset of allTasks
	byDep           map[string][]string
	byTrigger       map[string][]string
	byWatch         map[string][]string

	// Single message channel for the event loop.
	input chan any

	// Set once in Start, used for creating writers for dynamically
	// added tasks.
	out MultiWriter

	// Read-only after construction:
	allTasks  Tasks // full task universe
	runType   RunType
	rootID    string
	dir       string
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

const (
	internalTaskInterleaved = "@interleaved"
	internalTaskWatch       = "@watch"
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
// includes the IDs of each Task that will be used in the run, plus (if
// applicable) the id "@watch", which the Run uses for messaging about file
// watchers.
func (r *Run) IDs() []string {
	defer r.mu.Lock("IDs").Unlock()
	var ids []string
	if len(r.byWatch) > 0 {
		ids = append(ids, internalTaskWatch)
	}
	return append(ids, r.tasks.IDs()...)
}

// Tasks returns the Tasks that a Run would execute.
func (r *Run) Tasks() Tasks {
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
	r.out = out

	// Set up writers. Use a snapshot of IDs to avoid holding mu while
	// calling IDs() (which also takes mu).
	ids := r.IDs()
	r.mu.Lock("Start:writers")
	for _, id := range ids {
		r.writers[id] = newOutputWriter(out.Writer(id))
	}
	r.mu.Unlock()

	// Start all the file watchers. Do this before starting tasks so that
	// tasks can trigger file watcher events.
	for _, p := range r.watchedPaths() {
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
// exist in the allTasks collection passed to RunTask. Their transitive
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
		for _, depID := range r.byTrigger[id] {
			invalidations[depID] = struct{}{}
		}
	}

	// Invalidate "short" or unstarted tasks that list this as a
	// dependency and have all of their dependencies met.
	for _, depID := range r.byDep[id] {
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
					r.taskStatus[k] = TaskStatusFailed
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

		delay := time.Second * (1 << (attempts - 1))
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
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

	// If the task succeeded, signal readiness to start dependents.
	r.input <- msgTaskReady(msg.id)

	return nil
}

// handleFSEvent processes a file system event and restarts affected tasks.
func (r *Run) handleFSEvent(msg msgFSEvent) {
	r.printf(internalTaskWatch, logStyle, "%s", printFSEvent(msg))

	invalidations := map[string]struct{}{}
	for _, id := range r.byWatch[msg.path] {
		invalidations[id] = struct{}{}
	}
	if len(invalidations) > 0 {
		var ids []string
		for id := range invalidations {
			ids = append(ids, id)
		}
		r.printf(internalTaskWatch, logStyle, "invalidating {%s}", strings.Join(ids, ", "))
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
	case TaskStatusRunning, TaskStatusDone, TaskStatusFailed:
		r.input <- msgRunTask(id)
	}
}

// handleAddTasks activates tasks and their transitive dependencies, sets up
// watchers, creates writers, and starts zero-dep tasks.
func (r *Run) handleAddTasks(ctx context.Context, ids []string) {
	var newlyAdded []string

	// ingestTask recursively activates a task and its deps/triggers/watches.
	var ingestTask func(string)
	ingestTask = func(id string) {
		r.mu.Lock("handleAddTasks:check")
		_, alreadyActive := r.tasks.tasks[id]
		r.mu.Unlock()
		if alreadyActive {
			return
		}
		if !r.allTasks.Has(id) {
			return
		}

		t := r.allTasks.Get(id)

		r.mu.Lock("handleAddTasks:activate")
		r.tasks.tasks[id] = t
		r.tasks.ids = append(r.tasks.ids, id)
		r.taskStatus[id] = TaskStatusNotStarted
		for _, d := range t.Metadata().Triggers {
			r.byTrigger[d] = append(r.byTrigger[d], id)
		}
		for _, d := range t.Metadata().Dependencies {
			r.byDep[d] = append(r.byDep[d], id)
		}
		for _, w := range t.Metadata().Watch {
			r.byWatch[w] = append(r.byWatch[w], id)
		}
		if r.out != nil {
			r.writers[id] = newOutputWriter(r.out.Writer(id))
		}
		r.mu.Unlock()

		newlyAdded = append(newlyAdded, id)

		// Recurse into deps and triggers.
		for _, d := range t.Metadata().Dependencies {
			ingestTask(d)
		}
		for _, d := range t.Metadata().Triggers {
			ingestTask(d)
		}
	}

	for _, id := range ids {
		ingestTask(id)
	}

	// Start new file watchers for any watch paths that don't have one yet.
	r.mu.Lock("handleAddTasks:watches")
	var newWatchPaths []string
	for p := range r.byWatch {
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
	if len(r.byWatch) > 0 {
		if _, ok := r.writers[internalTaskWatch]; !ok && r.out != nil {
			r.writers[internalTaskWatch] = newOutputWriter(r.out.Writer(internalTaskWatch))
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

// handleRemoveTask deactivates a task and any exclusively-owned dependencies.
func (r *Run) handleRemoveTask(id string) {
	r.mu.Lock("handleRemoveTask:check")
	_, active := r.tasks.tasks[id]
	r.mu.Unlock()
	if !active {
		return
	}

	// Compute exclusively-owned tasks: tasks that are only needed by the
	// removed task and not by any other active task.
	toRemove := r.exclusivelyOwned(id)
	toRemove = append(toRemove, id)

	r.mu.Lock("handleRemoveTask:cleanup")
	for _, rid := range toRemove {
		// Cancel executor.
		if exec, ok := r.executors[rid]; ok {
			go exec.Cancel()
			delete(r.executors, rid)
		}

		// Remove from tasks.
		delete(r.tasks.tasks, rid)
		r.tasks.ids = removeFromSlice(r.tasks.ids, rid)

		// Clean up status maps.
		delete(r.taskStatus, rid)
		delete(r.restartAttempts, rid)
		delete(r.ran, rid)
		delete(r.writers, rid)

		// Clean up reverse-lookup maps.
		t := r.allTasks.Get(rid)
		if t != nil {
			for _, d := range t.Metadata().Dependencies {
				r.byDep[d] = removeFromSlice(r.byDep[d], rid)
			}
			for _, d := range t.Metadata().Triggers {
				r.byTrigger[d] = removeFromSlice(r.byTrigger[d], rid)
			}
			for _, w := range t.Metadata().Watch {
				r.byWatch[w] = removeFromSlice(r.byWatch[w], rid)
				// Stop watcher if no tasks watch this path.
				if len(r.byWatch[w]) == 0 {
					if stop, ok := r.watches[w]; ok {
						stop()
						delete(r.watches, w)
					}
					delete(r.byWatch, w)
				}
			}
		}
	}
	r.mu.Unlock()
}

// exclusivelyOwned returns task IDs that are transitive dependencies or
// triggers of `id` but not needed by any other active task.
func (r *Run) exclusivelyOwned(id string) []string {
	r.mu.Lock("exclusivelyOwned")
	defer r.mu.Unlock()

	t := r.tasks.tasks[id]
	if t == nil {
		return nil
	}

	// Collect all transitive deps/triggers of the task being removed.
	candidates := map[string]struct{}{}
	var collect func(string)
	collect = func(tid string) {
		task := r.tasks.tasks[tid]
		if task == nil {
			return
		}
		for _, d := range task.Metadata().Dependencies {
			if _, seen := candidates[d]; !seen {
				candidates[d] = struct{}{}
				collect(d)
			}
		}
		for _, d := range task.Metadata().Triggers {
			if _, seen := candidates[d]; !seen {
				candidates[d] = struct{}{}
				collect(d)
			}
		}
	}
	collect(id)

	// Check each candidate: is it needed by any other active task?
	var exclusive []string
	for cand := range candidates {
		needed := false
		// Check byDep: is any active task (other than id and other
		// candidates) depending on cand?
		for _, depOf := range r.byDep[cand] {
			if depOf == id {
				continue
			}
			if _, isCand := candidates[depOf]; isCand {
				continue
			}
			if _, isActive := r.tasks.tasks[depOf]; isActive {
				needed = true
				break
			}
		}
		if needed {
			continue
		}
		// Check byTrigger similarly.
		for _, trigOf := range r.byTrigger[cand] {
			if trigOf == id {
				continue
			}
			if _, isCand := candidates[trigOf]; isCand {
				continue
			}
			if _, isActive := r.tasks.tasks[trigOf]; isActive {
				needed = true
				break
			}
		}
		if !needed {
			exclusive = append(exclusive, cand)
		}
	}
	return exclusive
}

// startWatcher starts a file watcher for the given path and stores it in
// r.watches.
func (r *Run) startWatcher(p string) error {
	watchP := filepath.Join(r.dir, p)
	r.printf(internalTaskWatch, logStyle, "watching %s", watchP)
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

func (r *Run) printf(id string, style lipgloss.Style, f string, args ...interface{}) {
	r.mu.Lock("printf")
	w := r.writers[id]
	r.mu.Unlock()
	if w == nil {
		return
	}
	s := fmt.Sprintf(f, args...)
	w.Write([]byte(style.Render(s) + "\n"))
}

func (r *Run) watchedPaths() []string {
	var ps []string
	for p := range r.byWatch {
		ps = append(ps, p)
	}
	return ps
}

func removeFromSlice(s []string, val string) []string {
	out := s[:0]
	for _, v := range s {
		if v != val {
			out = append(out, v)
		}
	}
	return out
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
