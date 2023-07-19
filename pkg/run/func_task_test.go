package run_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run/pkg/run"
)

func TestFuncTask(t *testing.T) {
	exit, write, f := newControllableFunc()
	task := run.FuncTask(f, run.TaskMetadata{})
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		errs := make(chan error)
		go func() {
			err := task.Start(ctx, &b)
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

func newControllableFunc() (chan<- error, chan<- string, func(context.Context, io.Writer) error) {
	exit := make(chan error)
	write := make(chan string)

	f := func(c context.Context, w io.Writer) error {
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
