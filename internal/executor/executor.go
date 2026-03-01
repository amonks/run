// Package executor encapsulates running a cancelable function, canceling it,
// and waiting for it to finish.
package executor

import (
	"context"
	"sync/atomic"
)

var nextToken atomic.Int64

// Executor manages the lifecycle of a single cancelable function execution.
// It supports executing a function, canceling it synchronously, and waiting
// for it to complete. Each Executor has a unique token for identity comparison,
// enabling stale-goroutine detection.
type Executor struct {
	token  int64
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

// New creates a new Executor. The Executor is inert until Execute is called.
func New() *Executor {
	return &Executor{
		token: nextToken.Add(1),
		done:  make(chan struct{}),
	}
}

// Execute runs fn in a goroutine with a cancelable context derived from ctx.
// When fn returns, its error is stored and the Done channel is closed.
// Execute must be called exactly once per Executor.
func (e *Executor) Execute(ctx context.Context, fn func(context.Context) error) {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	go func() {
		e.err = fn(ctx)
		cancel()
		close(e.done)
	}()
}

// Cancel cancels the executor's context and blocks until the function exits.
// It returns the function's error. Cancel is safe to call multiple times;
// subsequent calls block until the function exits and return the same error.
func (e *Executor) Cancel() error {
	if e.cancel != nil {
		e.cancel()
	}
	<-e.done
	return e.err
}

// Done returns a channel that is closed when the executed function exits.
// Multiple goroutines can select on this channel simultaneously.
func (e *Executor) Done() <-chan struct{} {
	return e.done
}

// Err returns the error from the executed function. It is only valid to call
// Err after Done is closed.
func (e *Executor) Err() error {
	return e.err
}

// Is returns true if other is the same Executor (by token comparison).
// This is used for stale-goroutine detection: when a task exit message
// arrives, the caller can check whether the executor that exited is still
// the current one for that task.
func (e *Executor) Is(other *Executor) bool {
	return e.token == other.token
}
