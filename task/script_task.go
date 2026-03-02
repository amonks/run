package task

import (
	"context"
	"io"

	"github.com/amonks/run/internal/script"
)

// ScriptTask produces a runnable Task from a bash script and working
// directory. The script will execute in metadata.Dir. The script's Stdout and
// Stderr will be provided by the Run, and will be forwarded to the UI. The
// script will not get a Stdin.
//
// Script runs in a new bash process, and can have multiple lines. It is run
// basically like this:
//
//	$ cd $DIR
//	$ bash -c "$CMD" 2&>1 /some/ui
func ScriptTask(scriptText string, dir string, env []string, metadata TaskMetadata) Task {
	return &scriptTask{
		script:   script.Script{Dir: dir, Env: env, Text: scriptText},
		metadata: metadata,
	}
}

type scriptTask struct {
	script   script.Script
	metadata TaskMetadata
}

// *scriptTask implements Task
var _ Task = &scriptTask{}

// Dir returns the working directory the script will execute in.
func (t *scriptTask) Dir() string {
	return t.script.Dir
}

func (t *scriptTask) Metadata() TaskMetadata {
	return t.metadata
}

func (t *scriptTask) Start(ctx context.Context, onReady chan<- struct{}, stdout io.Writer) error {
	if t.script.Text == "" {
		// If this is a "long" task, we want to keep running until the
		// run is killed. Signal readiness immediately since there is
		// no script to wait on.
		// If this is a "short" task with no script, we should consider
		// it done as soon as its dependencies are.
		close(onReady)
		if t.metadata.Type == "long" {
			<-ctx.Done()
		}
		return nil
	}

	// For long tasks, signal readiness once the command starts
	// successfully.
	if t.metadata.Type == "long" {
		close(onReady)
	}

	err := t.script.Start(ctx, stdout, stdout)

	// For short tasks, signal readiness on successful exit.
	if t.metadata.Type != "long" && err == nil {
		close(onReady)
	}
	return err
}
