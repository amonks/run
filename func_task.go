package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// FuncTask produces a runnable Task from a go function. metadata.Dir is ignored.
func FuncTask(fn func(ctx context.Context, w io.Writer) error, metadata TaskMetadata) Task {
	return &funcTask{
		mu:       newMutex(fmt.Sprintf("script")),
		fn:       fn,
		metadata: metadata,
	}
}

type funcTask struct {
	mu *mutex

	fn       func(ctx context.Context, w io.Writer) error
	cancel   func()
	metadata TaskMetadata

	cmd     *exec.Cmd
	stdout  io.Writer
	waiters []chan<- error
}

// *funcTask implements Task
var _ Task = &funcTask{}

func (t *funcTask) Metadata() TaskMetadata {
	defer t.mu.Lock("Metadata").Unlock()

	meta := t.metadata
	return meta
}

func (t *funcTask) Start(stdout io.Writer) error {
	_ = t.Stop()
	defer t.mu.Lock("Start").Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.waiters = nil

	// Run the func!
	go func() {
		err := t.fn(ctx, stdout)
		t.notify(err)
	}()

	return nil
}

func (t *funcTask) Wait() <-chan error {
	defer t.mu.Lock("Wait").Unlock()

	c := make(chan error)
	t.waiters = append(t.waiters, c)
	return c
}

func (t *funcTask) notify(err error) {
	defer t.mu.Lock("notify").Unlock()

	for _, w := range t.waiters {
		select {
		case w <- err:
		default:
		}
		close(w)
	}
}

func (t *funcTask) Stop() error {
	waitC := t.Wait()
	defer t.mu.Lock("Stop").Unlock()

	t.cancel()
	err := <-waitC

	return err
}
