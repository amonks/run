package runner_test

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/amonks/run/runner"
	"github.com/amonks/run/taskfile"
)

// ui implements MultiWriter
var _ runner.MultiWriter = ui{}

type ui struct{}

func (w ui) Writer(string) io.Writer {
	return os.Stdout
}

// In this example, we build a version of the run CLI tool that uses a UI we
// provide ourselves.
func Example_bringYourOwnUI() {
	tasks, err := taskfile.Load(".")
	if err != nil {
		log.Fatal(err)
	}

	run, err := runner.New(".", tasks, "dev")
	if err != nil {
		log.Fatal(err)
	}

	ui := ui{}

	if err := run.Start(context.Background(), ui); err != nil {
		log.Fatal(err)
	}
}
