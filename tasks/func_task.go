package tasks

import (
	"context"
	"io"
)

// FuncTask wraps a function and some metadata into a Task. It's intended as a
// convenience for creating simple implementations of [Task].
type FuncTask struct {
	metadata TaskMetadata
	fn       func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error
}

// New turns some metadata and a start function into a task.
func NewTaskFromFunc(metadata TaskMetadata, fn func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error) FuncTask {
	return FuncTask{metadata, fn}
}

var _ Task = FuncTask{}

// Metadata implements [tasks.Task]. It returns the metadata, like
// dependencies, that will be used for orchestration by the runner.
func (t FuncTask) Metadata() TaskMetadata { return t.metadata }

// Start implements [tasks.Task].
func (t FuncTask) Start(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
	return t.fn(ctx, onReady, w)
}
