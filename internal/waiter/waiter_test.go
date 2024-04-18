package waiter_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/amonks/run/internal/fixtures"
	"github.com/amonks/run/internal/seq"
	"github.com/amonks/run/internal/waiter"
	"github.com/stretchr/testify/assert"
)

func TestWaiter(t *testing.T) {
	t.Run("start and wait", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task")
			w    = waiter.New(task.Thunk(&buf))
		)
		w.Start()
		err := <-w.Wait()
		assert.NoError(t, err)
		seq.AssertStringContainsSequence(t, buf.String(), "! task: execute")
	})

	t.Run("start and cancel", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithCancel(context.Canceled)
			w    = waiter.New(task.Thunk(&buf))
		)
		w.Start()
		err := w.Cancel()
		assert.ErrorIs(t, err, context.Canceled)
		seq.AssertStringContainsSequence(t, buf.String(), "! task: canceled")
	})

	t.Run("cancel closes waiter", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithCancel(context.Canceled)
			w    = waiter.New(task.Thunk(&buf))
		)
		w.Start()
		go func() { w.Cancel() }()
		err, ok := <-w.Wait()
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("wait again", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithImmediateFailure()
			w    = waiter.New(task.Thunk(&buf))
		)
		w.Start()
		assert.ErrorContains(t, <-w.Wait(), "fail")
		assert.ErrorContains(t, <-w.Wait(), "fail")
	})

	t.Run("start again", func(t *testing.T) {
		var (
			buf  bytes.Buffer
			task = fixtures.NewTask("task").WithImmediateFailure()
			w    = waiter.New(task.Thunk(&buf))
		)
		w.Start()
		assert.ErrorContains(t, <-w.Wait(), "fail")

		w.Start() // should no-op
		assert.ErrorContains(t, <-w.Wait(), "fail")
		assert.Equal(t, buf.String(), "! task: start\n! task: triggered failure\n")
	})
}
