package task_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"monks.co/run/task"
)

func TestFuncTask(t *testing.T) {
	exit, write, f := newControllableFunc()
	tk := task.FuncTask(f, task.TaskMetadata{})
	ctx := context.Background()

	for range 3 {
		b := strings.Builder{}

		errs := make(chan error)
		go func() {
			err := tk.Start(ctx, make(chan struct{}, 1), &b)
			errs <- err
		}()

		write <- "log"

		errExpected := errors.New("expected")

		done := make(chan struct{})
		go func() {
			err := <-errs
			if err != errExpected {
				t.Error("unexpected exit value", err)
			}
			done <- struct{}{}
		}()

		// Make sure we're waiting before we exit. 0.01ms is
		// consistently enough time on my machine, but let's play it
		// safe.
		time.Sleep(time.Millisecond)
		exit <- errExpected

		<-done

		if b.String() != "log" {
			t.Error("log output not logged")
		}
	}
}

func newControllableFunc() (chan<- error, chan<- string, func(context.Context, chan<- struct{}, io.Writer) error) {
	exit := make(chan error)
	write := make(chan string)

	f := func(c context.Context, onReady chan<- struct{}, w io.Writer) error {
		close(onReady)
		for {
			select {
			case str := <-write:
				w.Write([]byte(str))
			case err := <-exit:
				return err
			}
		}
	}

	return exit, write, f
}
