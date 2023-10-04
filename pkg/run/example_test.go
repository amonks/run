package run_test

import (
	"context"
	"os"

	"github.com/amonks/run/pkg/run"
)

// In this example, we use components from Run to build our own version of
// the run CLI tool. See cmd/run for the source of the -real- run CLI,
// which isn't too much more complex.
func Example() {
	tasks, _ := run.Load(".")
	r, _ := run.RunTask(tasks, "dev")
	ui := run.NewTUI(r)

	ctx := context.Background()
	uiReady := make(chan struct{})

	go ui.Start(ctx, uiReady, os.Stdin, os.Stdout)
	<-uiReady

	r.Start(ctx, ui) // blocks until done
}
