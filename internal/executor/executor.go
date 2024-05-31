// Executor takes the execution of a cancelable long-running function and wraps
// it into an object that can be passed around, canceled, and waited for.
package executor

import (
	"context"
	"sync"

	"github.com/amonks/run/internal/mutex"
)

type Executor struct {
	fn func(context.Context) error

	isDone bool

	ctx    context.Context
	cancel func()
	err    *error
	mu     *mutex.Mutex
	token  int

	started       bool
	canceled      chan<- error
	subscriptions []chan<- error
}

var tokenIncr = &incr{}

func New(fn func(ctx context.Context) error) *Executor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{fn: fn, ctx: ctx, cancel: cancel, mu: mutex.New("waiter"), token: tokenIncr.Incr()}
}

func (w *Executor) Is(other *Executor) bool {
	return w.token == other.token
}

func (w *Executor) Execute() {
	w.mu.Lock("Execute")
	defer w.mu.Unlock()

	if w.started {
		return
	}

	w.started = true

	go func() {
		err := w.fn(w.ctx)
		w.handleExit(err)
	}()
}

func (w *Executor) Wait() <-chan error {
	w.mu.Lock("Wait")
	defer w.mu.Unlock()
	if err := w.err; err != nil {
		c := make(chan error)
		go func() { c <- *err }()
		return c
	}

	c := make(chan error)
	w.subscriptions = append(w.subscriptions, c)

	return c
}

func (w *Executor) Cancel() error {
	return <-w.handleCancel()
}

func (w *Executor) IsDone() bool {
	return w.isDone
}

func (w *Executor) handleCancel() <-chan error {
	w.mu.Lock("handleCancel")
	defer w.mu.Unlock()

	c := make(chan error)
	w.isDone = true
	w.canceled = c
	w.cancel()
	return c
}

func (w *Executor) handleExit(err error) {
	w.mu.Lock("handleExit")
	defer w.mu.Unlock()

	w.err = &err
	w.isDone = true

	if w.canceled != nil {
		go func() { w.canceled <- err }()
		for _, sub := range w.subscriptions {
			close(sub)
		}
		return
	}

	for _, sub := range w.subscriptions {
		go func() { sub <- err }()
	}
}

type incr struct {
	n  int
	mu sync.Mutex
}

func (i *incr) Incr() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.n += 1
	return i.n
}
