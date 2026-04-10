package runner_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"monks.co/run/internal/fixtures"
	"monks.co/run/internal/watcher"
	"monks.co/run/runner"
	"monks.co/run/task"
)

// runTypeFor returns the RunType for a task library and root ID, matching
// the heuristic previously built into runner.New: long root → RunTypeLong.
func runTypeFor(tasks task.Library, rootID string) runner.RunType {
	if t := tasks.Get(rootID); t != nil && t.Metadata().Type == "long" {
		return runner.RunTypeLong
	}
	return runner.RunTypeShort
}

// startRun is a helper that creates and starts a run, returning the error on
// a channel. It also returns a cancel function to stop the run.
func startRun(t *testing.T, tasks []task.Task, rootID string, mw runner.MultiWriter) (context.CancelFunc, <-chan error) {
	t.Helper()

	lib := task.NewLibrary(tasks...)
	r, err := runner.New(runTypeFor(lib, rootID), ".", lib, rootID, mw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		errs <- r.Start(ctx)
	}()
	return cancel, errs
}

// startRunWithHandle is like startRun but also returns the *Run for dynamic
// Add/Remove/Invalidate.
func startRunWithHandle(t *testing.T, tasks []task.Task, rootID string, mw runner.MultiWriter) (*runner.Run, context.CancelFunc, <-chan error) {
	t.Helper()

	lib := task.NewLibrary(tasks...)
	r, err := runner.New(runTypeFor(lib, rootID), ".", lib, rootID, mw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		errs <- r.Start(ctx)
	}()
	return r, cancel, errs
}

// waitFor waits for the error channel with a timeout.
func waitFor(t *testing.T, errs <-chan error, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-errs:
		return err
	case <-time.After(timeout):
		t.Fatal("timed out waiting for run to complete")
		return nil
	}
}

// --- Test 1: Short run, no deps, succeeds ---

func TestShortRunNoDepsSucceeds(t *testing.T) {
	mw := fixtures.NewWriter()
	tk := fixtures.NewTask("build", "short")

	cancel, errs := startRun(t, []task.Task{tk}, "build", mw)
	defer cancel()

	err := waitFor(t, errs, 5*time.Second)
	assert.NoError(t, err)
	assert.Contains(t, mw.String("build"), "! build: execute")
}

// --- Test 2: Short run, no deps, fails ---

func TestShortRunNoDepsFails(t *testing.T) {
	mw := fixtures.NewWriter()
	tk := fixtures.NewTask("build", "short").
		WithImmediateFailure(errors.New("compile error"))

	cancel, errs := startRun(t, []task.Task{tk}, "build", mw)
	defer cancel()

	err := waitFor(t, errs, 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compile error")
}

// --- Test 3: Short run with dependency chain ---

func TestShortRunDependencyChain(t *testing.T) {
	mw := fixtures.NewWriter()
	t1 := fixtures.NewTask("gen", "short")
	t2 := fixtures.NewTask("compile", "short").WithDependencies("gen")
	t3 := fixtures.NewTask("link", "short").WithDependencies("compile")

	cancel, errs := startRun(t, []task.Task{t1, t2, t3}, "link", mw)
	defer cancel()

	err := waitFor(t, errs, 5*time.Second)
	assert.NoError(t, err)

	combined := mw.CombinedString()
	genIdx := strings.Index(combined, "[gen] ! gen: execute")
	compileIdx := strings.Index(combined, "[compile] ! compile: execute")
	linkIdx := strings.Index(combined, "[link] ! link: execute")
	assert.Greater(t, genIdx, -1, "gen should have executed")
	assert.Greater(t, compileIdx, -1, "compile should have executed")
	assert.Greater(t, linkIdx, -1, "link should have executed")
	assert.Less(t, genIdx, compileIdx, "gen should execute before compile")
	assert.Less(t, compileIdx, linkIdx, "compile should execute before link")
}

// --- Test 4: Failing dependency prevents dependent ---

func TestFailingDependencyPreventsDependents(t *testing.T) {
	mw := fixtures.NewWriter()
	t1 := fixtures.NewTask("gen", "short").
		WithImmediateFailure(errors.New("gen failed"))
	t2 := fixtures.NewTask("build", "short").WithDependencies("gen")

	cancel, errs := startRun(t, []task.Task{t1, t2}, "build", mw)
	defer cancel()

	err := waitFor(t, errs, 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gen failed")
	assert.NotContains(t, mw.String("build"), "! build:")
}

// --- Test 4b: Canceled sibling gets TaskStatusCanceled ---

func TestShortRunCanceledStatus(t *testing.T) {
	mw := fixtures.NewWriter()
	// "root" depends on both "fast-fail" and "slow". They run in parallel.
	// "fast-fail" fails immediately; "slow" should be canceled, not failed.
	t1 := fixtures.NewTask("fast-fail", "short").
		WithImmediateFailure(errors.New("boom"))
	t2 := fixtures.NewTask("slow", "short").WithCancel(context.Canceled)
	t3 := fixtures.NewTask("root", "short").WithDependencies("fast-fail", "slow")

	lib := task.NewLibrary(t1, t2, t3)
	r, err := runner.New(runner.RunTypeShort, ".", lib, "root", mw)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := t.Context()

	_ = r.Start(ctx)

	assert.Equal(t, runner.TaskStatusFailed, r.TaskStatus("fast-fail"),
		"the task that actually failed should be TaskStatusFailed")
	assert.Equal(t, runner.TaskStatusCanceled, r.TaskStatus("slow"),
		"a running task killed by another's failure should be TaskStatusCanceled")
}

// --- Test 5: Context cancellation ---

func TestContextCancellation(t *testing.T) {
	mw := fixtures.NewWriter()
	tk := fixtures.NewTask("server", "long").WithCancel(context.Canceled)

	cancel, errs := startRun(t, []task.Task{tk}, "server", mw)

	// Give the task time to start.
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := waitFor(t, errs, 5*time.Second)
	// Long run cancellation should return nil (clean shutdown).
	assert.NoError(t, err)
	assert.Contains(t, mw.String("server"), "! server: start")
}

// --- Test 6: Watch-triggered restart (long task) ---

func TestWatchTriggeredRestartLongTask(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	var startCount atomic.Int32
	tk := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		startCount.Add(1)
		w.Write([]byte("! server: start\n"))
		close(onReady)
		<-ctx.Done()
		return ctx.Err()
	}, task.TaskMetadata{
		ID:    "server",
		Type:  "long",
		Watch: []string{"src"},
	})

	mw := fixtures.NewWriter()
	_, cancel, errs := startRunWithHandle(t, []task.Task{tk}, "server", mw)
	defer cancel()

	// Wait for task to start.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), startCount.Load())

	// Dispatch a file-system event. The mock watcher key is
	// filepath.Join(".", "src") = "src".
	watcher.Dispatch("src", watcher.EventInfo{Path: "src/main.go", Event: "write"})

	// The event should trigger a restart (cancel + rerun).
	time.Sleep(200 * time.Millisecond)
	assert.GreaterOrEqual(t, startCount.Load(), int32(2), "task should have restarted after watch event")

	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 7: Trigger completion causes main task restart ---

func TestTriggerCompletionCausesRestart(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	// A short trigger with no watch: it runs once alongside the main
	// task. When it completes, the main task should be restarted.
	trigger := fixtures.NewTask("lint", "short")

	var startCount atomic.Int32
	main := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		startCount.Add(1)
		w.Write([]byte("! server: start\n"))
		close(onReady)
		<-ctx.Done()
		return ctx.Err()
	}, task.TaskMetadata{
		ID:       "server",
		Type:     "long",
		Triggers: []string{"lint"},
	})

	_, cancel, errs := startRunWithHandle(t, []task.Task{trigger, main}, "server", mw)

	// Let the run start and the trigger complete.
	time.Sleep(200 * time.Millisecond)

	// lint should have executed.
	assert.Contains(t, mw.String("lint"), "! lint: execute")
	// server should have been restarted by the trigger.
	assert.GreaterOrEqual(t, startCount.Load(), int32(2), "main task should restart when trigger completes")

	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 8: Watch event on trigger causes cascade restart ---

func TestWatchEventOnTriggerCausesCascade(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	trigger := fixtures.NewTask("codegen", "short").WithWatch("schema")

	var startCount atomic.Int32
	main := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		startCount.Add(1)
		w.Write([]byte("! app: start\n"))
		close(onReady)
		<-ctx.Done()
		return ctx.Err()
	}, task.TaskMetadata{
		ID:       "app",
		Type:     "long",
		Triggers: []string{"codegen"},
	})

	_, cancel, errs := startRunWithHandle(t, []task.Task{trigger, main}, "app", mw)
	defer cancel()

	// Wait for initial start: codegen runs once (auto-started as
	// zero-dep task), which triggers app restart.
	time.Sleep(200 * time.Millisecond)
	initialStartCount := startCount.Load()

	// Dispatch a watch event that should trigger codegen to rerun.
	watcher.Dispatch("schema", watcher.EventInfo{Path: "schema/api.graphql", Event: "write"})

	// Wait for the trigger to re-run and cascade.
	time.Sleep(300 * time.Millisecond)

	// codegen should have executed at least twice (initial + watch-triggered).
	assert.GreaterOrEqual(t, strings.Count(mw.String("codegen"), "! codegen: execute"), 2)
	// app should have been restarted again.
	assert.Greater(t, startCount.Load(), initialStartCount, "app should restart when trigger re-runs via watch")

	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 9: Dependency rerun does not restart long task ---

func TestDepRerunDoesNotRestartLongTask(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	dep := fixtures.NewTask("compile", "short").WithWatch("src")

	var startCount atomic.Int32
	main := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		startCount.Add(1)
		w.Write([]byte("! server: start\n"))
		close(onReady)
		<-ctx.Done()
		return ctx.Err()
	}, task.TaskMetadata{
		ID:           "server",
		Type:         "long",
		Dependencies: []string{"compile"},
	})

	_, cancel, errs := startRunWithHandle(t, []task.Task{dep, main}, "server", mw)
	defer cancel()

	// Wait for both to start.
	time.Sleep(100 * time.Millisecond)

	initialStartCount := startCount.Load()

	// Dispatch a watch event that triggers compile to rerun.
	watcher.Dispatch("src", watcher.EventInfo{Path: "src/main.go", Event: "write"})

	// Wait for rerun.
	time.Sleep(200 * time.Millisecond)

	cancel()
	waitFor(t, errs, 5*time.Second)

	// Compile should have run at least twice.
	assert.GreaterOrEqual(t, strings.Count(mw.String("compile"), "! compile: execute"), 2)
	// Server should NOT have been restarted.
	assert.Equal(t, initialStartCount, startCount.Load(), "long task should not restart when dependency reruns")
}

// --- Test 10: JSON output is prettified ---

func TestJSONOutputPrettified(t *testing.T) {
	mw := fixtures.NewWriter()
	tk := fixtures.NewTask("json-task", "short").
		WithOutput("{\"key\":\"value\",\"nested\":{\"a\":1}}\n")

	cancel, errs := startRun(t, []task.Task{tk}, "json-task", mw)
	defer cancel()

	err := waitFor(t, errs, 5*time.Second)
	assert.NoError(t, err)

	out := mw.String("json-task")
	// The output writer should have prettified the JSON.
	assert.Contains(t, out, "\"key\": \"value\"")
	assert.Contains(t, out, "  \"nested\"")
}

// --- Test 11: Long task onReady enables dependent ---

func TestLongTaskOnReadyEnablesDependent(t *testing.T) {
	mw := fixtures.NewWriter()

	ready := make(chan struct{})
	depExit := make(chan error, 1)

	dep := fixtures.NewTask("database", "long").
		WithReady(ready).
		WithExit(depExit)

	main := fixtures.NewTask("api", "short").WithDependencies("database")

	_, cancel, errs := startRunWithHandle(t, []task.Task{dep, main}, "api", mw)
	defer cancel()

	// api should not start until database signals ready.
	time.Sleep(100 * time.Millisecond)
	assert.NotContains(t, mw.String("api"), "! api:")

	// Signal readiness.
	close(ready)

	// api should now start.
	time.Sleep(100 * time.Millisecond)
	assert.Contains(t, mw.String("api"), "! api: execute")

	// Clean up: exit the database and cancel.
	depExit <- nil
	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 12: Dynamic Add ---

func TestDynamicAdd(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	main := fixtures.NewTask("server", "long").WithCancel(context.Canceled)
	extra := fixtures.NewTask("logger", "short")

	r, cancel, errs := startRunWithHandle(t, []task.Task{main, extra}, "server", mw)
	defer cancel()

	// Wait for run to start.
	time.Sleep(100 * time.Millisecond)

	// logger should not be running yet.
	assert.NotContains(t, mw.String("logger"), "! logger:")

	// Dynamically add it.
	r.Add("logger")

	time.Sleep(200 * time.Millisecond)
	assert.Contains(t, mw.String("logger"), "! logger: execute")

	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 13: Dynamic Remove ---

func TestDynamicRemove(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	dep := fixtures.NewTask("metrics", "long").WithCancel(context.Canceled)
	main := fixtures.NewTask("server", "long").
		WithCancel(context.Canceled).
		WithDependencies("metrics")

	r, cancel, errs := startRunWithHandle(t, []task.Task{dep, main}, "server", mw)
	defer cancel()

	// Wait for both to start.
	time.Sleep(100 * time.Millisecond)

	// Both should be running.
	assert.Contains(t, mw.String("metrics"), "! metrics: start")
	assert.Contains(t, mw.String("server"), "! server: start")

	// Remove metrics.
	r.Remove("metrics")

	time.Sleep(200 * time.Millisecond)

	// After removal, metrics should no longer be in the task list.
	assert.False(t, r.Tasks().Has("metrics"))

	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 14: Invalidate restarts running task ---

func TestInvalidateRestartsRunningTask(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	var startCount atomic.Int32
	tk := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		startCount.Add(1)
		w.Write([]byte("! worker: start\n"))
		close(onReady)
		<-ctx.Done()
		return ctx.Err()
	}, task.TaskMetadata{
		ID:   "worker",
		Type: "long",
	})

	r, cancel, errs := startRunWithHandle(t, []task.Task{tk}, "worker", mw)
	defer cancel()

	// Wait for task to start.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), startCount.Load())

	// Invalidate to trigger restart.
	r.Invalidate("worker")

	time.Sleep(200 * time.Millisecond)
	assert.GreaterOrEqual(t, startCount.Load(), int32(2), "task should have restarted after invalidation")

	cancel()
	waitFor(t, errs, 5*time.Second)
}

// --- Test 15: Watch-triggered restart (short dep in long run) ---

func TestWatchRestartShortDepInLongRun(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	dep := fixtures.NewTask("compile", "short").WithWatch("src")
	main := fixtures.NewTask("server", "long").
		WithCancel(context.Canceled).
		WithDependencies("compile")

	_, cancel, errs := startRunWithHandle(t, []task.Task{dep, main}, "server", mw)
	defer cancel()

	// Wait for initial run.
	time.Sleep(100 * time.Millisecond)
	assert.Contains(t, mw.String("compile"), "! compile: execute")

	// Dispatch a watch event.
	watcher.Dispatch("src", watcher.EventInfo{Path: "src/main.go", Event: "write"})

	// Wait for the compile task to be rerun.
	time.Sleep(200 * time.Millisecond)

	cancel()
	waitFor(t, errs, 5*time.Second)

	// Compile should have run at least twice.
	assert.GreaterOrEqual(t, strings.Count(mw.String("compile"), "! compile: execute"), 2)
}

// --- Test 16: Watch event does not bypass dependency check ---

func TestWatchEventRespectsUnmetDependencies(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	mw := fixtures.NewWriter()

	// install is a short task that takes a while to complete.
	// We use a channel to control when it finishes.
	installDone := make(chan struct{})
	install := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		fmt.Fprintf(w, "! install: execute\n")
		select {
		case <-installDone:
			close(onReady)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}, task.TaskMetadata{
		ID:   "install",
		Type: "short",
	})

	// build depends on install and watches a path.
	var buildCount atomic.Int32
	build := task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
		buildCount.Add(1)
		fmt.Fprintf(w, "! build: execute\n")
		close(onReady)
		return nil
	}, task.TaskMetadata{
		ID:           "build",
		Type:         "short",
		Dependencies: []string{"install"},
		Watch:        []string{"css"},
	})

	_, cancel, errs := startRunWithHandle(t, []task.Task{install, build}, "build", mw)
	defer cancel()

	// Wait for install to start.
	time.Sleep(100 * time.Millisecond)
	assert.Contains(t, mw.String("install"), "! install: execute")

	// Dispatch a watch event matching build's watch pattern while
	// install is still running (has not exited).
	watcher.Dispatch("css", watcher.EventInfo{Path: "css/index.css", Event: "write"})

	// build should NOT start because install hasn't completed.
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), buildCount.Load(),
		"build must not start while its dependency (install) is still running")

	// Now let install complete.
	close(installDone)

	// build should run.
	time.Sleep(200 * time.Millisecond)
	assert.GreaterOrEqual(t, buildCount.Load(), int32(1),
		"build should run after install completes")

	cancel()
	waitFor(t, errs, 5*time.Second)
}
