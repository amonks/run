package runner_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/amonks/runner"
)

// In this example, we generate our own Task and run it.
func Example_bringYourOwnTasks() {
	tasks := runner.Tasks{
		"custom": runner.FuncTask(func(ctx context.Context, w io.Writer) error {
			w.Write([]byte("sleep"))
			time.Sleep(1 * time.Second)
			w.Write([]byte("done"))
			return nil
		}, runner.TaskMetadata{}),
	}

	run, err := runner.RunTask(".", tasks, "custom")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(strings.Join(run.IDs(), ", "))
	// Output: runner, custom
}
