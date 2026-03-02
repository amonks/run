package runner_test

import (
	"context"
	"os"

	"github.com/amonks/run/runner"
	"github.com/amonks/run/taskfile"
	"github.com/amonks/run/tui"
)

// In this example, we use components from Run to build our own version of
// the run CLI tool. See the root package for the source of the -real- run CLI,
// which isn't too much more complex.
func Example() {
	tasks, _ := taskfile.Load(".")
	r, _ := runner.New(".", tasks, "dev")
	ui := tui.New(r)

	ctx := context.Background()
	uiReady := make(chan struct{})

	go ui.Start(ctx, uiReady, os.Stdin, os.Stdout)
	<-uiReady

	r.Start(ctx, ui) // blocks until done
}
