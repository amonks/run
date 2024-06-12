package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"reflect"
	"strings"

	meta "github.com/amonks/run"
	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/printer"
	"github.com/amonks/run/runner"
	"github.com/amonks/run/taskfile"
	"github.com/amonks/run/tasks"
	"github.com/amonks/run/tui"
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
	fPprof        = flag.Bool("pprof", false, "Run a pprof server on port 6060.")
)

func main() {
	flag.Parse()

	if *fPprof {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

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

	allTasks, err := taskfile.Load(*fDir)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}
	library := allTasks.ToLibrary()

	if *fList {
		fmt.Println(tasklistText(library))
		os.Exit(0)
	}

	taskID := flag.Arg(0)
	if taskID == "" {
		fmt.Println(helpText())
		os.Exit(0)
	}

	subtree := library.Subtree(taskID)

	var useTUI bool
	switch *fUI {
	case "tui":
		useTUI = true
	case "printer":
		useTUI = false
	case "":
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			useTUI = false
		} else if subtree.HasAnyLongTask() {
			useTUI = true
		} else {
			useTUI = false
		}
	default:
		fmt.Println("Invalid value for flag -ui. Legal values are 'tui' and 'printer'.")
		os.Exit(1)
	}

	if useTUI {
		tui.Start(context.Background(), os.Stdin, os.Stdout, library, taskID)
	} else {
		prn := printer.New(subtree.LongestID(), os.Stdout)
		r := runner.New(runner.RunnerModeExit, library, *fDir, prn)
		r.Run(context.Background(), taskID)
	}
}

func tasklistText(tasks tasks.Library) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, headerStyle.Render("TASKS"))
	for i, id := range tasks.IDs() {
		if i != 0 {
			b.WriteString("\n")
		}
		t := tasks.Task(id)
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
