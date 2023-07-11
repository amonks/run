package run_test

import (
	"log"
	"os"
	"sync"

	"github.com/amonks/run"
)

// In this example, we use components from Run to build our own version of
// the run CLI tool. See cmd/run for the source of the -real- run CLI,
// which isn't too much more complex.
func Example() {
	tasks, err := run.Load(".")
	if err != nil {
		log.Fatal(err)
	}

	r, err := run.RunTask(".", tasks, "dev")
	if err != nil {
		log.Fatal(err)
	}

	ui := run.NewTUI()
	ui.Start(os.Stdin, os.Stdout, r.IDs())
	r.Start(ui)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-ui.Wait(); err != nil {
			log.Fatal(err)
		}
		r.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-r.Wait(); err != nil {
			log.Fatal(err)
		}
		ui.Stop()
	}()

	wg.Wait()
}
