package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amonks/run"
	"github.com/carlmjohnson/versioninfo"
	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/term"
)

var (
	fChosenUI  = flag.String("ui", "", "Force a particular ui. Legal values are 'tui' and 'printer'.")
	fChosenDir = flag.String("dir", ".", "Look for a root taskfile in the given directory.")
	fList      = flag.Bool("list", false, "Display the task list and exit. If run is invoked with both -list and a task ID, that task's dependencies are displayed.")
	fVersion   = flag.Bool("version", false, "Display the version and exit.")
	fHelp      = flag.Bool("help", false, "Display the help text and exit.")
	fCredits   = flag.Bool("credits", false, "Display the open source credits and exit.")
	fLicense   = flag.Bool("license", false, "Display the license info and exit.")
)

func main() {
	flag.Parse()

	if *fVersion {
		fmt.Println("\n" + versionText())
		os.Exit(0)
	} else if *fHelp {
		fmt.Println("\n" + helpText())
		os.Exit(0)
	} else if *fCredits {
		fmt.Println("\n" + creditsText())
		os.Exit(0)
	} else if *fLicense {
		fmt.Println("\n" + licenseText())
		os.Exit(0)
	}

	allTasks, err := run.Load(*fChosenDir)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}

	taskID := flag.Arg(0)
	if taskID == "" {
		if *fList {
			fmt.Println(tasklistText(allTasks.IDs()))
			os.Exit(0)
		}
		fmt.Println(helpText())
		os.Exit(0)
	}

	r, err := run.RunTask(*fChosenDir, allTasks, taskID)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	if *fList {
		fmt.Println(tasklistText(r.IDs()[1:]))
		os.Exit(0)
	}

	var ui run.UI
	switch *fChosenUI {
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

func tasklistText(ids []string) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "")
	fmt.Fprintln(b, "TASKS")
	for _, id := range ids {
		b.WriteString("  - " + id + "\n")
	}
	return b.String()
}

func helpText() string {
	b := &strings.Builder{}
	b.WriteString("USAGE\n")
	b.WriteString("  run [flags] <task>\n")
	b.WriteString("\n")
	b.WriteString("Run executes collections of tasks defined in tasks.toml files.\n")
	b.WriteString("For documentation and the latest version, please visit GitHub:\n")
	b.WriteString("\n")
	b.WriteString("  https://github.com/amonks/run\n")
	b.WriteString("\n")
	b.WriteString(flagText())
	b.WriteString("\n")
	b.WriteString(versionText())
	b.WriteString("\n")
	b.WriteString(shortLicenseText())
	b.WriteString("\n")
	b.WriteString("  run with -license for more info\n")
	b.WriteString("\n")

	return b.String()
}

func flagText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "FLAGS")
	flag.CommandLine.SetOutput(b)
	flag.PrintDefaults()
	flag.CommandLine.SetOutput(os.Stdout)
	return b.String()
}

func versionText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "VERSION")
	fmt.Fprintln(b, "  Version:", versioninfo.Version)
	fmt.Fprintln(b, "  Revision:", versioninfo.Revision)
	if versioninfo.Revision != "unknown" {
		fmt.Fprintln(b, "  Committed:", versioninfo.LastCommit.Format(time.RFC1123))
		if versioninfo.DirtyBuild {
			fmt.Fprintln(b, "  Dirty Build")
		}
	}
	return b.String()
}

//go:generate go run github.com/amonks/run/cmd/licenses credits.txt
//go:embed credits.txt
var credits string

func creditsText() string {
	return "CREDITS\n\n" + indent.String(wordwrap.String(credits, 78), 2)
}

//go:generate cp ../../LICENSE.md ./LICENSE.md
//go:embed LICENSE.md
var license string
var statement = "Run is free for noncommercial and small-business use, with a guarantee that fair, reasonable, and nondiscriminatory paid-license terms will be available for everyone else."

func licenseText() string {
	return shortLicenseText() + "\n\n\n" +
		indent.String(wordwrap.String(license, 60), 2) + "\n"
}

func shortLicenseText() string {
	b := &strings.Builder{}
	b.WriteString("LICENSE\n")
	b.WriteString("\n")
	b.WriteString("  © Andrew Monks <a@monks.co>\n")
	b.WriteString("\n")
	b.WriteString(indent.String(wordwrap.String(statement, 60), 2))
	return b.String() + "\n"
}
