package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/amonks/runner"
	"github.com/carlmjohnson/versioninfo"
	"golang.org/x/term"
)

var (
	chosenUI  = flag.String("ui", "", "Force a particular ui. Legal values are 'tui', 'colorful-printer', 'colorless-printer'")
	chosenDir = flag.String("dir", ".", "Look for a root taskfile in the given directory")
)

func main() {
	versioninfo.AddFlag(nil)
	flag.Parse()

	allTasks, err := runner.Load(*chosenDir)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}

	taskID := os.Args[len(os.Args)-1]
	run, err := runner.RunTask(*chosenDir, allTasks, taskID)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	var ui runner.UI
	switch *chosenUI {
	case "tui":
		ui = runner.NewTUI()
	case "printer":
		ui = runner.NewPrinter()
	case "":
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			ui = runner.NewPrinter()
		} else if run.Type() == runner.RunTypeShort {
			ui = runner.NewPrinter()
		} else {
			ui = runner.NewTUI()
		}
	}

	if err := ui.Start(os.Stdin, os.Stdout, run.IDs()); err != nil {
		fmt.Println("Error starting run:")
		fmt.Println(err)
		os.Exit(1)
	}

	if err := run.Start(ui); err != nil {
		ui.Stop()
		fmt.Println("Error starting UI:")
		fmt.Println(err)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-ui.Wait(); err != nil {
			fmt.Println("UI failed:")
			fmt.Println(err)
		}
		run.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-run.Wait(); err != nil {
			fmt.Println("Run failed:")
			fmt.Println(err)
		}
		ui.Stop()
	}()

	wg.Wait()
}
