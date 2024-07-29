package fixtures

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/amonks/run/tasks"
)

type Task struct {
	meta tasks.TaskMetadata

	output   []string
	onCancel *error

	ready <-chan struct{}
	exit  <-chan error
}

var _ tasks.Task = &Task{}

func NewTask(id string) *Task { return &Task{meta: tasks.TaskMetadata{ID: id, Type: "short"}} }

func (t *Task) Metadata() tasks.TaskMetadata { return t.meta }

func (t *Task) WithType(typ string) *Task           { t.meta.Type = typ; return t }
func (t *Task) WithWatch(ws ...string) *Task        { t.meta.Watch = ws; return t }
func (t *Task) WithDependencies(ds ...string) *Task { t.meta.Dependencies = ds; return t }
func (t *Task) WithTriggers(ts ...string) *Task     { t.meta.Triggers = ts; return t }

func (t *Task) WithOutput(output ...string) *Task     { t.output = output; return t }
func (t *Task) WithReady(ready <-chan struct{}) *Task { t.ready = ready; return t }
func (t *Task) WithExit(ex <-chan error) *Task        { t.exit = ex; return t }
func (t *Task) WithCancel(err error) *Task            { t.onCancel = &err; return t }

func (t *Task) WithImmediateFailure() *Task {
	c := make(chan error)
	go func() { c <- errors.New("fail") }()
	t.WithExit(c)
	return t
}

func (t *Task) Start(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
	if t.onCancel != nil {
		fmt.Fprintf(w, "! %s: start\n", t.meta.ID)
		<-ctx.Done()
		fmt.Fprintf(w, "! %s: canceled\n", t.meta.ID)
		return *t.onCancel
	}

	if t.exit == nil {
		if t.output == nil {
			fmt.Fprintf(w, "! %s: execute\n", t.meta.ID)
		} else {
			for _, s := range t.output {
				fmt.Fprint(w, s)
			}
		}
		return nil
	}

	fmt.Fprintf(w, "! %s: start\n", t.meta.ID)
	if t.ready != nil {
		<-t.ready
		fmt.Fprintf(w, "! %s: ready\n", t.meta.ID)
	}

	if err := <-t.exit; err != nil {
		fmt.Fprintf(w, "! %s: triggered failure\n", t.meta.ID)
		return err
	} else {
		fmt.Fprintf(w, "! %s: triggered success\n", t.meta.ID)
		return nil
	}
}

func (t *Task) Thunk(w io.Writer) func(context.Context) error {
	return func(ctx context.Context) error {
		return t.Start(ctx, make(chan struct{}), w)
	}
}
