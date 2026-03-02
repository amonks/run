package runner_test

import (
	"context"
	"os"

	"github.com/amonks/run/taskfile"
	"github.com/amonks/run/tui"
)

// In this example, we use components from Run to build our own version of
// the run CLI tool. See the root package for the source of the -real- run CLI,
// which isn't too much more complex.
func Example() {
	tasks, _ := taskfile.Load(".")
	tui.Start(context.Background(), os.Stdin, os.Stdout, ".", tasks, "dev")
}
