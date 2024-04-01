package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"

	meta "github.com/amonks/run"
	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/pkg/run"
	"github.com/muesli/reflow/dedent"
	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/term"
)

var (
	fUI   = flag.String("ui", "", "Force a particular ui. Legal values are 'tui' and 'printer'.")
	fDir  = flag.String("dir", ".", "Look for a root taskfile in the given directory.")
	fList = flag.Bool("list", false, "Display the task list and exit. If run is invoked with both -list and a task ID, that task's dependencies are displayed.")

	fVersion      = flag.Bool("version", false, "Display the version and exit.")
	fHelp         = flag.Bool("help", false, "Display the help text and exit.")
	fCredits      = flag.Bool("credits", false, "Display the open source credits and exit.")
	fContributors = flag.Bool("contributors", false, "Display the contributors list and exit.")
	fLicense      = flag.Bool("license", false, "Display the license info and exit.")
)

func main() {
	flag.Parse()

	if *fVersion {
		fmt.Println(versionText())
		os.Exit(0)
	} else if *fHelp {
		fmt.Println("\n" + helpText())
		os.Exit(0)
	} else if *fCredits {
		fmt.Print(creditsText())
		os.Exit(0)
	} else if *fContributors {
		fmt.Print(contributorsText())
		os.Exit(0)
	} else if *fLicense {
		fmt.Println("\n" + licenseText())
		os.Exit(0)
	}

	allTasks, err := run.Load(*fDir)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}

	taskID := flag.Arg(0)
	if taskID == "" {
		if *fList {
			fmt.Println(tasklistText(allTasks))
			os.Exit(0)
		}
		fmt.Println(helpText())
		os.Exit(0)
	}

	r, err := run.RunTask(*fDir, allTasks, taskID)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	if *fList {
		fmt.Println(tasklistText(r.Tasks()))
		os.Exit(0)
	}

	var ui run.UI
	switch *fUI {
	case "tui":
		ui = run.NewTUI(r)
	case "printer":
		ui = run.NewPrinter(r)
	case "":
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			ui = run.NewPrinter(r)
		} else if r.Type() == run.RunTypeShort {
			ui = run.NewPrinter(r)
		} else {
			ui = run.NewTUI(r)
		}
	default:
		fmt.Println("Invalid value for flag -ui. Legal values are 'tui' and 'printer'.")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)

	uiReady := make(chan struct{})

	// Whether the UI or the Run exits first, that first exit is the cause
	// of program exit, so we want to capture its error and base the exit
	// code on it. The second exit is just a side effect of the first thing
	// dying, so we don't need it.
	exitReason := &first[error]{}

	go func() {
		defer wg.Done()
		err := ui.Start(ctx, uiReady, os.Stdin, os.Stdout)
		if !exitReason.isSet() {
			if err != nil {
				exitReason.set(err)
			} else if r.Type() == run.RunTypeShort {
				// If the UI exits before the run, and the run
				// is short, that itself is an error even if the
				// ui returns nil.
				exitReason.set(errors.New("UI exited before run was complete"))
			} else {
				// exit ok
				exitReason.set(context.Canceled)
			}
		}
		if err != context.Canceled {
			cancel()
		}
	}()

	<-uiReady

	go func() {
		defer wg.Done()
		err := r.Start(ctx, ui)
		exitReason.set(err)

		// Don't close the UI if '-tui' was explicitly set--the user
		// indicated that they want to look carefully at output.
		if *fUI != "tui" && err != context.Canceled {
			cancel()
		}
	}()

	allDone := make(chan struct{})
	go func() { wg.Wait(); allDone <- struct{}{} }()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	select {
	case <-sigs:
		cancel()
		<-allDone
	case <-allDone:
	}

	if err := exitReason.get(); err != nil && errors.Is(err, context.Canceled) {
		fmt.Printf("Canceled\n")
		os.Exit(0)
	} else if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

func tasklistText(tasks run.Tasks) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("TASKS"))
	var ids []string
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for i, id := range ids {
		if i != 0 {
			b.WriteString("\n")
		}
		t := tasks[id]
		meta := t.Metadata()

		fmt.Fprintf(b, "  %s\n", color.RenderHash(id))
		fmt.Fprintf(b, "    Type: %s\n", italicStyle.Render(meta.Type))
		if meta.Description != "" {
			fmt.Fprintf(b, "    Description:\n")
			desc := strings.TrimRight(dedent.String(meta.Description), "\n")
			b.WriteString(indent.String(italicStyle.Render(desc), 6) + "\n")
		}
		if len(meta.Dependencies) != 0 {
			fmt.Fprintf(b, "    Dependencies:\n")
			for _, dep := range meta.Dependencies {
				fmt.Fprintf(b, "      - %s\n", dep)
			}
		}
		if len(meta.Triggers) != 0 {
			fmt.Fprintf(b, "    Triggers:\n")
			for _, dep := range meta.Triggers {
				fmt.Fprintf(b, "      - %s\n", dep)
			}
		}
		if len(meta.Watch) != 0 {
			fmt.Fprintf(b, "    Watch:\n")
			for _, dep := range meta.Watch {
				fmt.Fprintf(b, "      - %s\n", dep)
			}
		}
	}
	return b.String()
}

func init() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, usageText())
		fmt.Fprintln(w, flagText())
		os.Exit(0)
	}
}

func helpText() string {
	b := &strings.Builder{}
	b.WriteString("Run executes collections of tasks defined in tasks.toml files.\n")
	b.WriteString("For documentation and the latest version, please visit GitHub:\n")
	b.WriteString("\n")
	b.WriteString("  https://github.com/amonks/run\n")
	b.WriteString("\n")
	b.WriteString(usageText())
	b.WriteString("\n")
	b.WriteString(flagText())
	b.WriteString("\n")
	b.WriteString(versionText())
	b.WriteString("\n")
	b.WriteString(shortLicenseText())

	return b.String()
}

func usageText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("USAGE"))
	b.WriteString("  run [flags] <task>\n")
	return b.String()
}

func flagText() string {
	var b strings.Builder
	fmt.Fprintln(&b, headerStyle.Render("FLAGS"))

	f := flag.CommandLine

	f.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(&b, "  -%s", f.Name) // Two spaces before -; see next two comments.
		name, usage := flag.UnquoteUsage(f)
		if len(name) > 0 {
			b.WriteString("=")
			b.WriteString(name)
		}
		// Print the default value only if it differs to the zero value
		// for this flag type.
		if isZero := isZeroValue(f, f.DefValue); !isZero {
			fmt.Fprintf(&b, " (default %q)", f.DefValue)
		}
		b.WriteString("\n")

		usage = strings.ReplaceAll(usage, "\n", "\n    \t")
		usage = wordwrap.String(usage, 52)
		usage = indent.String(usage, 8)
		b.WriteString(usage)

		b.WriteString("\n")
	})
	return b.String()
}

// isZeroValue determines whether the string represents the zero
// value for a flag.
func isZeroValue(f *flag.Flag, value string) (ok bool) {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Pointer {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	return value == z.Interface().(flag.Value).String()
}
func versionText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("VERSION"))
	fmt.Fprintln(b, "  Version:", meta.Version)
	if meta.Revision != "unknown" {
		if meta.DirtyBuild {
			fmt.Fprintln(b, "  Dirty Build")
			fmt.Fprintln(b, "  Last commit:", meta.ReleaseDate)
		} else {
			fmt.Fprintln(b, "  Revision:", meta.Revision)
			fmt.Fprintln(b, "  Committed:", meta.ReleaseDate)
		}
	}
	return b.String()
}

func creditsText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("CREDITS"))
	fmt.Fprintln(b, indent.String(wordwrap.String(meta.Credits, 78), 2))
	return b.String()
}

func contributorsText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("CONTRIBUTORS"))
	fmt.Fprintln(b, indent.String(wordwrap.String(meta.Contributors, 78), 2))
	return b.String()
}

var statement = "Run is free for noncommercial and small-business use, with a guarantee that fair, reasonable, and nondiscriminatory paid-license terms will be available for everyone else. Ask about paid licenses at a@monks.co."

func licenseText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("LICENSE"))
	b.WriteString("  © Andrew Monks <a@monks.co>\n")
	b.WriteString("\n")
	b.WriteString(indent.String(wordwrap.String(statement, 70), 2) + "\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(indent.String(wordwrap.String(meta.License, 70), 2))
	return b.String()
}

func shortLicenseText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("LICENSE"))
	b.WriteString(indent.String(wordwrap.String(statement, 60), 2) + "\n")
	b.WriteString("\n")
	b.WriteString("  Run `run -license` for more info.\n")
	b.WriteString("\n")
	b.WriteString("  © Andrew Monks <a@monks.co>\n")
	return b.String()
}
