package task_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"monks.co/run/task"
)

// In this example, we generate our own Task and run it.
func Example_bringYourOwnTasks() {
	tasks := task.NewLibrary(
		task.FuncTask(func(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
			w.Write([]byte("sleep"))
			time.Sleep(1 * time.Second)
			w.Write([]byte("done"))
			close(onReady)
			return nil
		}, task.TaskMetadata{ID: "custom", Type: "short"}),
	)

	ids := tasks.IDs()
	if len(ids) != 1 {
		log.Fatal("expected 1 task")
	}

	fmt.Println(strings.Join(ids, ", "))
	// Output: custom
}
