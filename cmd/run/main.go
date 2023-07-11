package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/amonks/run"
	"github.com/carlmjohnson/versioninfo"
	"golang.org/x/term"
)

var (
	chosenUI  = flag.String("ui", "", "Force a particular ui. Legal values are 'tui' and 'printer'.")
	chosenDir = flag.String("dir", ".", "Look for a root taskfile in the given directory.")
)

func main() {
	versioninfo.AddFlag(nil)
	flag.Parse()

	allTasks, err := run.Load(*chosenDir)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}

	taskID := flag.Arg(0)
	r, err := run.RunTask(*chosenDir, allTasks, taskID)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	var ui run.UI
	switch *chosenUI {
	case "tui":
		ui = run.NewTUI()
	case "printer":
		ui = run.NewPrinter()
	case "":
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			ui = run.NewPrinter()
		} else if r.Type() == run.RunTypeShort {
			ui = run.NewPrinter()
		} else {
			ui = run.NewTUI()
		}
	default:
		fmt.Println("Invalid value for flag -ui. Legal values are 'tui' and 'printer'.")
		os.Exit(1)
	}

	if err := ui.Start(os.Stdin, os.Stdout, r.IDs()); err != nil {
		fmt.Println("Error starting run:")
		fmt.Println(err)
		os.Exit(1)
	}

	if err := r.Start(ui); err != nil {
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
		r.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := <-r.Wait(); err != nil {
			fmt.Println("Run failed:")
			fmt.Println(err)
		}
		ui.Stop()
	}()

	wg.Wait()
}
