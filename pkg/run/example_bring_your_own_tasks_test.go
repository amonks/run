package run_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/amonks/run/pkg/run"
)

// In this example, we generate our own Task and run it.
func Example_bringYourOwnTasks() {
	tasks := run.Tasks{
		"custom": run.FuncTask(func(ctx context.Context, w io.Writer) error {
			w.Write([]byte("sleep"))
			time.Sleep(1 * time.Second)
			w.Write([]byte("done"))
			return nil
		}, run.TaskMetadata{ID: "custom", Type: "short"}),
	}

	run, err := run.RunTask(tasks, "custom")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(strings.Join(run.IDs(), ", "))
	// Output: run, custom
}
