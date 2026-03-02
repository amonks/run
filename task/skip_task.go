package task

import (
	"context"
	"io"
)

// SkipTask wraps an existing Task, replacing its Start behavior with a stub
// that prints "skipping" and either exits immediately (short) or blocks until
// context cancellation (long). The original task's metadata is preserved so
// that the dependency graph remains valid.
func SkipTask(original Task) Task {
	return &skipTask{metadata: original.Metadata()}
}

type skipTask struct {
	metadata TaskMetadata
}

var _ Task = &skipTask{}

func (t *skipTask) Metadata() TaskMetadata {
	return t.metadata
}

func (t *skipTask) Start(ctx context.Context, onReady chan<- struct{}, stdout io.Writer) error {
	stdout.Write([]byte("skipping\n"))
	close(onReady)
	if t.metadata.Type == "long" {
		<-ctx.Done()
	}
	return nil
}
