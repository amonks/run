package runner_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run/internal/fixtures"
	"github.com/amonks/run/internal/seq"
	"github.com/amonks/run/internal/watcher"
	"github.com/amonks/run/runner"
	"github.com/amonks/run/tasks"
	"github.com/stretchr/testify/assert"
)

const waitTime = time.Millisecond * 100

func TestRunner(t *testing.T) {
	t.Run("exit run with no dependencies succeeds", func(t *testing.T) {
		var (
			task = fixtures.NewTask("task")
			lib  = tasks.NewLibrary(task)
			mw   = fixtures.NewWriter()
			r    = runner.New(runner.RunnerModeExit, lib, ".", mw)
			ctx  = context.Background()
		)
		err := r.Run(ctx, "task")
		assert.NoError(t, err)
		assert.Equal(t, join(
			"[task] starting",
			"[task] ! task: execute",
			"[task] exit ok",
			"[@interleaved] done",
		), mw.CombinedString())
	})

	t.Run("exit run with no dependencies fails", func(t *testing.T) {
		var (
			task = fixtures.NewTask("task").WithImmediateFailure()
			lib  = tasks.NewLibrary(task)
			mw   = fixtures.NewWriter()
			r    = runner.New(runner.RunnerModeExit, lib, ".", mw)
			ctx  = context.Background()
		)
		err := r.Run(ctx, "task")
		assert.Error(t, err)
		assert.Equal(t, join(
			"[task] starting",
			"[task] ! task: start",
			"[task] ! task: triggered failure",
			"[task] exit: fail",
			"[@interleaved] done",
		), mw.CombinedString())
	})

	t.Run("exit run with dependencies succeeds", func(t *testing.T) {
		var (
			lib = tasks.NewLibrary(
				fixtures.NewTask("1"),
				fixtures.NewTask("2").WithDependencies("1"),
				fixtures.NewTask("3").WithDependencies("2", "1"),
			)
			mw  = fixtures.NewWriter()
			r   = runner.New(runner.RunnerModeExit, lib, ".", mw)
			ctx = context.Background()
		)
		err := r.Run(ctx, "3")
		output := strings.Split(mw.CombinedString(), "\n")
		assert.NoError(t, err)
		seq.AssertContainsSequence(t, output,
			"[1] ! 1: execute",
			"[2] ! 2: execute",
			"[3] ! 3: execute",
		)
	})

	t.Run("exit run has failing dependency", func(t *testing.T) {
		var (
			lib = tasks.NewLibrary(
				fixtures.NewTask("failing-task").WithImmediateFailure(),
				fixtures.NewTask("task").WithDependencies("failing-task"),
			)
			mw  = fixtures.NewWriter()
			r   = runner.New(runner.RunnerModeExit, lib, ".", mw)
			ctx = context.Background()
		)
		err := r.Run(ctx, "task")
		assert.Error(t, err)
		lines := strings.Split(mw.CombinedString(), "\n")
		seq.AssertContainsSequence(t, lines, "[failing-task] exit: fail")
		assert.NotContains(t, lines, "[task] ! task: execute")
	})

	t.Run("exit run with long task is canceled", func(t *testing.T) {
		var (
			task        = fixtures.NewTask("long").WithCancel(errors.New("canceled"))
			lib         = tasks.NewLibrary(task)
			mw          = fixtures.NewWriter()
			r           = runner.New(runner.RunnerModeExit, lib, ".", mw)
			ctx, cancel = context.WithCancel(context.Background())
		)
		go func() { time.Sleep(waitTime); cancel() }()
		err := r.Run(ctx, "long")
		assert.ErrorIs(t, err, context.Canceled)
		seq.AssertStringContainsSequence(t, mw.CombinedString(),
			"[long] ! long: start",
			"[@interleaved] run canceled",
			"[@interleaved] done",
		)
	})

	t.Run("keepalive run with long task restarts on watch change", func(t *testing.T) {
		watcher.Mock()
		defer watcher.Unmock()

		var (
			task        = fixtures.NewTask("long").WithCancel(errors.New("canceled")).WithWatch("path")
			lib         = tasks.NewLibrary(task)
			mw          = fixtures.NewWriter()
			r           = runner.New(runner.RunnerModeKeepalive, lib, ".", mw)
			ctx, cancel = context.WithCancel(context.Background())
			errs        = make(chan error)
		)
		go func() { errs <- r.Run(ctx, "long") }()
		time.Sleep(waitTime)
		watcher.Dispatch("path")
		time.Sleep(waitTime)

		cancel()
		assert.ErrorIs(t, <-errs, context.Canceled)
		seq.AssertStringContainsSequence(t, mw.CombinedString(),
			"[long] ! long: start",
			"[long] ! long: canceled",
			"[long] ! long: start",
			"[@interleaved] run canceled",
			"[long] ! long: canceled",
		)
	})

	t.Run("keepalive run with short task restarts on watch change", func(t *testing.T) {
		watcher.Mock()
		defer watcher.Unmock()

		var (
			task        = fixtures.NewTask("short").WithType("short").WithWatch("path")
			lib         = tasks.NewLibrary(task)
			mw          = fixtures.NewWriter()
			r           = runner.New(runner.RunnerModeKeepalive, lib, ".", mw)
			ctx, cancel = context.WithCancel(context.Background())
			errs        = make(chan error)
		)
		go func() { errs <- r.Run(ctx, "short") }()
		time.Sleep(waitTime)
		watcher.Dispatch("path")
		time.Sleep(waitTime)

		cancel()
		assert.ErrorIs(t, <-errs, context.Canceled)
		seq.AssertStringContainsSequence(t, mw.CombinedString(),
			"[short] ! short: execute",
			"[short] ! short: execute",
			"[@interleaved] run canceled",
		)
	})

	t.Run("unlike dependencies, triggers should not be automatically started", func(t *testing.T) {
		var (
			task        = fixtures.NewTask("long").WithCancel(errors.New("canceled")).WithTriggers("dep")
			dep         = fixtures.NewTask("dep")
			lib         = tasks.NewLibrary(task, dep)
			mw          = fixtures.NewWriter()
			r           = runner.New(runner.RunnerModeKeepalive, lib, ".", mw)
			ctx, cancel = context.WithCancel(context.Background())
			errs        = make(chan error)
		)
		go func() { errs <- r.Run(ctx, "long") }()
		time.Sleep(waitTime)
		cancel()
		assert.ErrorIs(t, <-errs, context.Canceled)
		seq.AssertStringContainsSequence(t, mw.CombinedString(),
			"[long] ! long: start",
			"[@interleaved] run canceled",
			"[long] ! long: canceled",
		)
		assert.NotContains(t, "[dep]", mw.CombinedString())
	})

	t.Run("long task should be restarted by its trigger rerunning", func(t *testing.T) {
		var (
			task        = fixtures.NewTask("long").WithCancel(errors.New("canceled")).WithTriggers("dep")
			dep         = fixtures.NewTask("dep")
			lib         = tasks.NewLibrary(task, dep)
			mw          = fixtures.NewWriter()
			r           = runner.New(runner.RunnerModeKeepalive, lib, ".", mw)
			ctx, cancel = context.WithCancel(context.Background())
			errs        = make(chan error)
		)
		go func() { errs <- r.Run(ctx, "long") }()
		time.Sleep(waitTime)
		r.Invalidate("dep")
		time.Sleep(waitTime)

		cancel()
		assert.ErrorIs(t, <-errs, context.Canceled)
		seq.AssertStringContainsSequence(t, mw.CombinedString(),
			"[long] ! long: start",
			"[dep] ! dep: execute",
			"[long] ! long: canceled",
			"[long] ! long: start",
			"[@interleaved] run canceled",
			"[long] ! long: canceled",
			"[@interleaved] done",
		)
	})

	t.Run("long task should not be restarted by its dependency rerunning", func(t *testing.T) {
		var (
			task        = fixtures.NewTask("long").WithCancel(errors.New("canceled")).WithDependencies("dep")
			dep         = fixtures.NewTask("dep")
			lib         = tasks.NewLibrary(task, dep)
			mw          = fixtures.NewWriter()
			r           = runner.New(runner.RunnerModeKeepalive, lib, ".", mw)
			ctx, cancel = context.WithCancel(context.Background())
			errs        = make(chan error)
		)
		go func() { errs <- r.Run(ctx, "long") }()
		time.Sleep(waitTime)
		r.Invalidate("dep")
		time.Sleep(waitTime)

		cancel()
		assert.ErrorIs(t, <-errs, context.Canceled)
		seq.AssertStringContainsSequence(t, mw.CombinedString(),
			"[dep] ! dep: execute",
			"[long] ! long: start",
			"[dep] manually invalidated",
			"[dep] ! dep: execute",
			"[@interleaved] run canceled",
			"[@interleaved] done",
		)
	})

	t.Run("json output is prettified", func(t *testing.T) {
		var (
			task = fixtures.NewTask("task").WithOutput(`{"obj":{"key":"value","n":5}}` + "\n")
			lib  = tasks.NewLibrary(task)
			mw   = fixtures.NewWriter()
			r    = runner.New(runner.RunnerModeExit, lib, ".", mw)
			ctx  = context.Background()
		)
		err := r.Run(ctx, "task")
		assert.NoError(t, err)
		assert.Equal(t, join(
			"[task] starting",
			`[task] {`,
			`  "obj": {`,
			`    "key": "value",`,
			`    "n": 5`,
			`  }`,
			`}`,
			"[task] exit ok",
			"[@interleaved] done",
		), mw.CombinedString())
	})

	t.Run("env is inherited", func(t *testing.T) {
		// This is tested in :/tasks/script/script_task_test.go
	})

	t.Run("dir", func(t *testing.T) {
		// This is tested in :/tasks/script/script_task_test.go
	})
}

func join(ss ...string) string {
	return strings.Join(ss, "\n") + "\n"
}
