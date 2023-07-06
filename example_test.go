package runner_test

import (
	"log"
	"os"
	"sync"

	"github.com/amonks/runner"
)

// In this example, we use components from Runner to build our own version of
// the runner CLI tool. See cmd/runner for the source of the -real- runner CLI,
// which isn't too much more complex.
func Example() {
	tasks, err := runner.Load(".")
	if err != nil {
		log.Fatal(err)
	}

	run, err := runner.RunTask(".", tasks, "dev")
	if err != nil {
		log.Fatal(err)
	}

	ui := runner.NewTUI()
	ui.Start(os.Stdin, os.Stdout, run.IDs())
	run.Start(ui)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-ui.Wait(); err != nil {
			log.Fatal(err)
		}
		run.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-run.Wait(); err != nil {
			log.Fatal(err)
		}
		ui.Stop()
	}()

	wg.Wait()
}
