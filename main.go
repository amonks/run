package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"monks.co/run/internal/color"
	"monks.co/run/printer"
	"monks.co/run/runner"
	"monks.co/run/task"
	"monks.co/run/taskfile"
	"monks.co/run/tui"
	"github.com/muesli/reflow/dedent"
	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/term"
)

var (
	fUI   = flag.String("ui", "", "Force a particular ui. Legal values are 'tui' and 'printer'.")
	fDir  = flag.String("dir", ".", "Look for a root taskfile in the given directory.")
	fList = flag.Bool("list", false, "Display the task list and exit. If run is invoked with both -list and a task ID, that task's dependencies are displayed.")
	fSkip []string

	fVersion      = flag.Bool("version", false, "Display the version and exit.")
	fHelp         = flag.Bool("help", false, "Display the help text and exit.")
	fCredits      = flag.Bool("credits", false, "Display the open source credits and exit.")
	fContributors = flag.Bool("contributors", false, "Display the contributors list and exit.")
	fLicense      = flag.Bool("license", false, "Display the license info and exit.")
)

func init() {
	flag.Func("skip", "Skip a task, replacing it with a no-op stub. Can be passed more than once.", func(s string) error {
		fSkip = append(fSkip, s)
		return nil
	})
}

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

	taskID := flag.Arg(0)

	allTasks, err := taskfile.Load(*fDir, taskID)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}

	if len(fSkip) > 0 {
		for _, id := range fSkip {
			if !allTasks.Has(id) {
				fmt.Printf("Cannot skip %q: task not found.\n", id)
				os.Exit(1)
			}
		}
		skipSet := map[string]struct{}{}
		for _, id := range fSkip {
			skipSet[id] = struct{}{}
		}
		var tasks []task.Task
		for _, id := range allTasks.IDs() {
			t := allTasks.Get(id)
			if _, ok := skipSet[id]; ok {
				t = task.SkipTask(t)
			}
			tasks = append(tasks, t)
		}
		allTasks = task.NewLibrary(tasks...)
	}

	if taskID == "" {
		if *fList {
			fmt.Println(tasklistText(allTasks))
			os.Exit(0)
		}
		fmt.Println(helpText())
		os.Exit(0)
	}

	if !allTasks.Has(taskID) {
		fmt.Printf("Task %q not found.\n", taskID)
		fmt.Println("Run `run -list` for more information about the available tasks.")
		os.Exit(1)
	}

	if *fList {
		fmt.Println(tasklistText(allTasks.Subtree(taskID)))
		os.Exit(0)
	}

	// Determine UI mode.
	useTUI := false
	switch *fUI {
	case "tui":
		useTUI = true
	case "printer":
		useTUI = false
	case "":
		if term.IsTerminal(int(os.Stdout.Fd())) {
			if allTasks.Get(taskID).Metadata().Type == "long" {
				useTUI = true
			}
		}
	default:
		fmt.Println("Invalid value for flag -ui. Legal values are 'tui' and 'printer'.")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	var runErr error
	if useTUI {
		runErr = tui.Start(ctx, os.Stdin, os.Stdout, *fDir, allTasks, taskID)
	} else {
		subtree := allTasks.Subtree(taskID)
		prn := printer.New(subtree.LongestID(), os.Stdout)
		r, err := runner.New(runner.RunTypeShort, *fDir, allTasks, taskID, prn)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		runErr = r.Start(ctx)
	}

	if runErr != nil && errors.Is(runErr, context.Canceled) {
		fmt.Printf("Canceled\n")
		os.Exit(0)
	} else if runErr != nil {
		fmt.Printf("Error: %s\n", runErr)
		os.Exit(1)
	}
}

func tasklistText(tasks task.Library) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("TASKS"))
	for i, id := range tasks.IDs() {
		if i != 0 {
			b.WriteString("\n")
		}
		t := tasks.Get(id)
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
	b.WriteString("  https://monks.co/run\n")
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
	fmt.Fprintln(b, "  Version:", Version)
	if Revision != "unknown" {
		if DirtyBuild {
			fmt.Fprintln(b, "  Dirty Build")
			fmt.Fprintln(b, "  Last commit:", ReleaseDate)
		} else {
			fmt.Fprintln(b, "  Revision:", Revision)
			fmt.Fprintln(b, "  Committed:", ReleaseDate)
		}
	}
	return b.String()
}

func creditsText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("CREDITS"))
	fmt.Fprintln(b, indent.String(wordwrap.String(Credits, 78), 2))
	return b.String()
}

func contributorsText() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("CONTRIBUTORS"))
	fmt.Fprintln(b, indent.String(wordwrap.String(Contributors, 78), 2))
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
	b.WriteString(indent.String(wordwrap.String(License, 70), 2))
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
