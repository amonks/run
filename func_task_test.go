package run_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run"
)

func TestFuncTask(t *testing.T) {
	exit, write, f := newControllableFunc()
	task := run.FuncTask(f, run.TaskMetadata{})

	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		if err := task.Start(&b); err != nil {
			t.Fatal(err)
		}

		select {
		case <-task.Wait():
			t.Fatal("task exited unexpectedly")
		default:
		}

		write <- "log"

		errExpected := errors.New("expected")

		waited := make(chan struct{})
		go func() {
			err := <-task.Wait()
			if err != errExpected {
				t.Error("unexpected exit value", err)
			}
			waited <- struct{}{}
		}()

		// Make sure we're waiting before we exit. 0.01ms is
		// consistently enough time on my machine, but let's play it
		// safe.
		time.Sleep(time.Millisecond)
		exit <- errExpected

		<-waited

		if b.String() != "log" {
			t.Error("log output nont logged")
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
