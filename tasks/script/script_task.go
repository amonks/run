package script

import (
	"context"
	"io"

	"github.com/amonks/run/internal/script"
	"github.com/amonks/run/tasks"
)

type Task struct {
	metadata tasks.TaskMetadata
	script   script.Script
}

// New creates a new Script Task with the given working directory, environment,
// and text. If dir is the empty string, the script is run in the current
// working directory. Env is appended to the current environment. Script is
// evaluated in a new bash process. Effectivley, it is equivalent to
//
//	$ cd $DIR $ $ENV bash -c "$TEXT"
func New(metadata tasks.TaskMetadata, dir string, env map[string]string, text string) Task {
	return Task{
		metadata: metadata,
		script:   script.New(dir, env, text),
	}
}

// Dir returns the directory that the script will execute in.
func (t Task) Dir() string { return t.script.Dir }

var _ tasks.Task = Task{}

// Metadata implements [tasks.Task]. It returns the metadata, like
// dependencies, that will be used for orchestration by the runner.
func (t Task) Metadata() tasks.TaskMetadata { return t.metadata }

// Start implements [tasks.Task]. It executes the script and returns immediately.
// until the script is done executing. The returned error will be nil only if
// the process exits with status code 0 and is not interrupted by a context
// cancelation.
//
// Execution can be canceled with the provided context. When canceled, we first
// send SIGINT, then if the process doesn't exit within 2 seconds, we send
// SIGKILL. Start will always return an error if the context is canceled before
// the script is complete.
func (t Task) Start(ctx context.Context, onReady chan<- struct{}, w io.Writer) error {
	// If this is a "long" task, we want to keep running until the
	// run is killed. If this is a "short" task with no script, we
	// should consider it done as soon as its dependencies are.
	if t.script.Text == "" {
		if t.Metadata().Type == "long" {
			<-ctx.Done()
		}
		return nil
	}

	// Mark long tasks as ready immediately
	if t.Metadata().Type == "long" {
		onReady <- struct{}{}
	}

	err := t.script.Start(ctx, w, w)
	if t.Metadata().Type != "long" {
		close(onReady)
	}
	return err
}
