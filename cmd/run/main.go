package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"
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
	fUI   = flag.String("ui", "", "Force a particular ui. Legal values are 'tui' and 'printer'.")
	fDir  = flag.String("dir", ".", "Look for a root taskfile in the given directory.")
	fList = flag.Bool("list", false, "Display the task list and exit. If run is invoked with both -list and a task ID, that task's dependencies are displayed.")

	fVersion = flag.Bool("version", false, "Display the version and exit.")
	fHelp    = flag.Bool("help", false, "Display the help text and exit.")
	fCredits = flag.Bool("credits", false, "Display the open source credits and exit.")
	fLicense = flag.Bool("license", false, "Display the license info and exit.")
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

	allTasks, err := run.Load(*fDir)
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

	r, err := run.RunTask(*fDir, allTasks, taskID)
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
	switch *fUI {
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
	b.WriteString("USAGE\n")
	b.WriteString("  run [flags] <task>\n")
	return b.String()
}

func flagText() string {
	var b strings.Builder
	fmt.Fprintln(&b, "FLAGS")

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

func creditsText() string {
	return "CREDITS\n\n" + indent.String(wordwrap.String(run.Credits, 78), 2)
}

var statement = "Run is free for noncommercial and small-business use, with a guarantee that fair, reasonable, and nondiscriminatory paid-license terms will be available for everyone else."

func licenseText() string {
	b := &strings.Builder{}
	b.WriteString("LICENSE\n")
	b.WriteString("\n")
	b.WriteString("  © Andrew Monks <a@monks.co>\n")
	b.WriteString("\n")
	b.WriteString(indent.String(wordwrap.String(statement, 70), 2) + "\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(indent.String(wordwrap.String(run.License, 70), 2))
	return b.String()
}

func shortLicenseText() string {
	b := &strings.Builder{}
	b.WriteString("LICENSE\n")
	b.WriteString(indent.String(wordwrap.String(statement, 60), 2) + "\n")
	b.WriteString("\n")
	b.WriteString("  Run `run -license` for more info.\n")
	b.WriteString("\n")
	b.WriteString("  © Andrew Monks <a@monks.co>\n")
	return b.String()
}
