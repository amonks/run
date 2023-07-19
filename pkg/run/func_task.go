package run

import (
	"context"
	"io"
)

// FuncTask produces a runnable Task from a go function. metadata.Dir is ignored.
func FuncTask(fn func(ctx context.Context, w io.Writer) error, metadata TaskMetadata) Task {
	return &funcTask{
		fn:       fn,
		metadata: metadata,
	}
}

type funcTask struct {
	fn func(ctx context.Context, w io.Writer) error

	// read-only
	metadata TaskMetadata
}

// *funcTask implements Task
var _ Task = &funcTask{}

func (t *funcTask) Metadata() TaskMetadata {
	meta := t.metadata
	return meta
}

func (t *funcTask) Start(ctx context.Context, stdout io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan error)

	// Run the func!
	go func() {
		exit <- t.fn(ctx, stdout)
	}()

	select {
	case err := <-exit:
		cancel()
		return err
	case <-ctx.Done():
		cancel()
		return <-exit
	}
}
