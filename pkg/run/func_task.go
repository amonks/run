package run

import (
	"context"
	"io"
)

// FuncTask produces a runnable Task from a go function. The function receives
// an onReady channel that it should close when it is ready (i.e., has produced
// whatever output dependents need). metadata.Dir is ignored.
func FuncTask(fn func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error, metadata TaskMetadata) Task {
	return &funcTask{
		fn:       fn,
		metadata: metadata,
	}
}

type funcTask struct {
	fn func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error

	// read-only
	metadata TaskMetadata
}

// *funcTask implements Task
var _ Task = &funcTask{}

func (t *funcTask) Metadata() TaskMetadata {
	meta := t.metadata
	return meta
}

func (t *funcTask) Start(ctx context.Context, onReady chan<- struct{}, stdout io.Writer) error {
	exit := make(chan error)

	// Run the func!
	go func() {
		exit <- t.fn(ctx, onReady, stdout)
	}()

	select {
	case err := <-exit:
		return err
	case <-ctx.Done():
		return <-exit
	}
}
