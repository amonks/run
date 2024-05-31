package executor_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/amonks/run/internal/executor"
	"github.com/amonks/run/internal/fixtures"
	"github.com/amonks/run/internal/seq"
	"github.com/stretchr/testify/assert"
)

func TestExecutor(t *testing.T) {
	t.Run("start and wait", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task")
			w    = executor.New(task.Thunk(&buf))
		)
		w.Execute()
		err := <-w.Wait()
		assert.NoError(t, err)
		seq.AssertStringContainsSequence(t, buf.String(), "! task: execute")
	})

	t.Run("start and cancel", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithCancel(context.Canceled)
			w    = executor.New(task.Thunk(&buf))
		)
		w.Execute()
		err := w.Cancel()
		assert.ErrorIs(t, err, context.Canceled)
		seq.AssertStringContainsSequence(t, buf.String(), "! task: canceled")
	})

	t.Run("cancel closes executor", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithCancel(context.Canceled)
			w    = executor.New(task.Thunk(&buf))
		)
		w.Execute()
		go func() { w.Cancel() }()
		err, ok := <-w.Wait()
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("wait again", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithImmediateFailure()
			w    = executor.New(task.Thunk(&buf))
		)
		w.Execute()
		assert.ErrorContains(t, <-w.Wait(), "fail")
		assert.ErrorContains(t, <-w.Wait(), "fail")
	})

	t.Run("start again", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithImmediateFailure()
			w    = executor.New(task.Thunk(&buf))
		)
		w.Execute()
		assert.ErrorContains(t, <-w.Wait(), "fail")

		w.Execute() // should no-op
		assert.ErrorContains(t, <-w.Wait(), "fail")
		assert.Equal(t, buf.String(), "! task: start\n! task: triggered failure\n")
	})
}
