package run_test

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/amonks/run/pkg/run"
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

	ctx, cancel := context.WithCancel(context.Background())

	ui := run.NewTUI(r)

	var wg sync.WaitGroup
	ready := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ui.Start(ctx, ready, os.Stdin, os.Stdout, r.IDs()); err != nil {
			log.Fatal(err)
		}
		cancel()
	}()

	<-ready

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.Start(ctx, ui); err != nil {
			log.Fatal(err)
		}
		cancel()
	}()

	wg.Wait()
}
