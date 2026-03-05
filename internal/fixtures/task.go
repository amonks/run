// Package fixtures provides test doubles for the run package.
package fixtures

import (
	"context"
	"fmt"
	"io"

	"monks.co/run/task"
)

// Task is a controllable mock implementation of task.Task for testing.
type Task struct {
	id           string
	taskType     string
	watch        []string
	dependencies []string
	triggers     []string
	output       string

	// Channels for controlling the task's lifecycle.
	ready  <-chan struct{} // close to signal readiness
	exit   <-chan error    // send error to exit
	cancel error          // error to return on cancellation

	// If true, Start returns an error immediately.
	immediateFail error
}

// NewTask creates a new mock task with the given ID and type.
func NewTask(id, taskType string) *Task {
	return &Task{id: id, taskType: taskType}
}

func (t *Task) WithWatch(watch ...string) *Task {
	cp := *t
	cp.watch = watch
	return &cp
}

func (t *Task) WithDependencies(deps ...string) *Task {
	cp := *t
	cp.dependencies = deps
	return &cp
}

func (t *Task) WithTriggers(triggers ...string) *Task {
	cp := *t
	cp.triggers = triggers
	return &cp
}

func (t *Task) WithOutput(output string) *Task {
	cp := *t
	cp.output = output
	return &cp
}

// WithReady configures the task to wait for a readiness signal. The caller
// should close the returned channel to signal readiness.
func (t *Task) WithReady(ch <-chan struct{}) *Task {
	cp := *t
	cp.ready = ch
	return &cp
}

// WithExit configures the task to block until an error is sent on ch. Send
// nil for a clean exit.
func (t *Task) WithExit(ch <-chan error) *Task {
	cp := *t
	cp.exit = ch
	return &cp
}

// WithCancel configures the task to block on ctx.Done() and return err when
// canceled.
func (t *Task) WithCancel(err error) *Task {
	cp := *t
	cp.cancel = err
	return &cp
}

// WithImmediateFailure configures Start to return err immediately.
func (t *Task) WithImmediateFailure(err error) *Task {
	cp := *t
	cp.immediateFail = err
	return &cp
}

func (t *Task) Metadata() task.TaskMetadata {
	return task.TaskMetadata{
		ID:           t.id,
		Type:         t.taskType,
		Watch:        t.watch,
		Dependencies: t.dependencies,
		Triggers:     t.triggers,
	}
}

func (t *Task) Start(ctx context.Context, onReady chan<- struct{}, stdout io.Writer) error {
	if t.immediateFail != nil {
		return t.immediateFail
	}

	if t.exit != nil {
		// Long-lived task: start, optionally wait for ready signal,
		// then wait for exit.
		fmt.Fprintf(stdout, "! %s: start\n", t.id)

		if t.output != "" {
			fmt.Fprint(stdout, t.output)
		}

		if t.ready != nil {
			select {
			case <-t.ready:
				close(onReady)
			case <-ctx.Done():
				fmt.Fprintf(stdout, "! %s: canceled\n", t.id)
				return ctx.Err()
			}
		}

		select {
		case err := <-t.exit:
			if err == nil {
				close(onReady)
			}
			return err
		case <-ctx.Done():
			fmt.Fprintf(stdout, "! %s: canceled\n", t.id)
			return ctx.Err()
		}
	}

	if t.cancel != nil {
		// Blocking task: start, then wait for cancellation.
		fmt.Fprintf(stdout, "! %s: start\n", t.id)

		if t.output != "" {
			fmt.Fprint(stdout, t.output)
		}

		close(onReady)
		<-ctx.Done()
		fmt.Fprintf(stdout, "! %s: canceled\n", t.id)
		return t.cancel
	}

	// Default: execute immediately and return nil.
	fmt.Fprintf(stdout, "! %s: execute\n", t.id)
	if t.output != "" {
		fmt.Fprint(stdout, t.output)
	}
	close(onReady)
	return nil
}

// Verify Task implements task.Task.
var _ task.Task = (*Task)(nil)
