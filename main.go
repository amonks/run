package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"monks.co/run/internal/color"
	"monks.co/run/printer"
	"monks.co/run/runner"
	"monks.co/run/session"
	"monks.co/run/task"
	"monks.co/run/taskfile"
	"monks.co/run/tui"
	"github.com/muesli/reflow/dedent"
	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/term"
)

// --- Invocation types ---

type RunInvocation struct {
	Task string   `pos:"0" required:"true"`
	Dir  string   `flag:"dir" default:"." usage:"Look for a root taskfile in the given directory."`
	Skip []string `flag:"skip" usage:"Skip a task, replacing it with a no-op stub. Can be passed more than once."`
	UI   string   `flag:"ui" usage:"Force a particular ui. Legal values are 'tui' and 'printer'."`
}

type InspectInvocation struct {
	Task string `pos:"0"`
	Dir  string `flag:"dir" default:"." usage:"Look for a root taskfile in the given directory."`
	List bool   `flag:"list" required:"true" usage:"Display the task list and exit. If run is invoked with both -list and a task ID, that task's dependencies are displayed."`
}

type SessionInvocation struct {
	Dir     string `flag:"dir" default:"." usage:"Look for a root taskfile in the given directory."`
	Name    string `flag:"session" required:"true" usage:"The name of the task you started (e.g. dev)."`
	Status  bool   `flag:"status" usage:"Show whether each task is running, failed, or done."`
	Restart string `flag:"restart" usage:"Restart a task. Useful after changing code."`
	Log     string `flag:"log" usage:"Write a task's output to a log file. The file path is printed to stdout and is based on the task name (e.g. ~/.local/share/run/.../logs/build.log). Read it with cat, tail -f, or any other tool."`
	Nolog   string `flag:"nolog" usage:"Stop writing a task's output to its log file."`
}

type InfoInvocation struct {
	Version      bool `flag:"version" usage:"Display the version and exit."`
	Help         bool `flag:"help" usage:"Display the help text and exit."`
	License      bool `flag:"license" usage:"Display the license info and exit."`
	Credits      bool `flag:"credits" usage:"Display the open source credits and exit."`
	Contributors bool `flag:"contributors" usage:"Display the contributors list and exit."`
}

// --- Mode definitions ---

var runInv RunInvocation
var inspectInv InspectInvocation
var sessionInv SessionInvocation
var infoInv InfoInvocation

var modes = []mode{
	{
		name:        "RUNNING TASKS",
		description: "Execute a task and its dependencies.",
		usage:       "run [flags] <task>",
		inv:         &runInv,
	},
	{
		name: "INTERACTING WITH RUNNING TASKS",
		description: "In addition to the TUI, you can check on and control long-running tasks from the command line. Use -session with the name of the task you started (e.g. if you ran `run dev`, use -session=dev).",
		examples: []string{
			"run -session=<name> -status",
			"run -session=<name> -restart=<task>",
			"run -session=<name> -log=<task>",
			"run -session=<name> -nolog=<task>",
		},
		inv: &sessionInv,
	},
	{
		name:        "INSPECTING THE TASKFILE",
		description: "Display information about the taskfile.",
		examples: []string{
			"run -list",
			"run -list <task>",
		},
		inv: &inspectInv,
	},
	{
		name: "ABOUT",
		inv:  &infoInv,
		after: func() string {
			return versionInline() + "\n" + licenseInline()
		},
	},
}

func init() {
	registerFlags(modes)
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "\n"+renderHelp(modes))
	}
}

func main() {
	flag.Parse()
	syncSharedFlags(modes)

	m := resolveMode(modes)
	if m == nil {
		fmt.Println(renderHelp(modes))
		os.Exit(0)
	}
	setPositional(m)

	switch m.inv.(type) {
	case *InfoInvocation:
		handleInfo()
	case *InspectInvocation:
		handleInspect()
	case *SessionInvocation:
		handleSession()
	case *RunInvocation:
		handleRun()
	}
}

func handleInfo() {
	switch {
	case infoInv.Version:
		fmt.Println(versionText())
	case infoInv.Help:
		fmt.Println("\n" + renderHelp(modes))
	case infoInv.License:
		fmt.Println("\n" + licenseText())
	case infoInv.Credits:
		fmt.Print(creditsText())
	case infoInv.Contributors:
		fmt.Print(contributorsText())
	}
}

func handleInspect() {
	allTasks, err := taskfile.Load(inspectInv.Dir, inspectInv.Task)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}
	if inspectInv.Task != "" {
		if !allTasks.Has(inspectInv.Task) {
			fmt.Printf("Task %q not found.\n", inspectInv.Task)
			fmt.Println("Run `run -list` for more information about the available tasks.")
			os.Exit(1)
		}
		fmt.Println(tasklistText(allTasks.Subtree(inspectInv.Task)))
	} else {
		fmt.Println(tasklistText(allTasks))
	}
}

func handleSession() {
	absDir, err := filepath.Abs(sessionInv.Dir)
	if err != nil {
		fmt.Printf("Error resolving directory: %s\n", err)
		os.Exit(1)
	}

	client, err := session.Connect(sessionInv.Name, absDir)
	if err != nil {
		fmt.Printf("Error connecting to session '%s': %s\n", sessionInv.Name, err)
		os.Exit(1)
	}

	switch {
	case sessionInv.Status:
		status, err := client.Status()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		if term.IsTerminal(int(os.Stdout.Fd())) {
			for _, t := range status.Tasks {
				logIndicator := ""
				if t.Log {
					logIndicator = "  [log]"
				}
				fmt.Printf("%-40s %s%s\n", t.ID, t.Status, logIndicator)
			}
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(status)
		}

	case sessionInv.Log != "":
		path, err := client.EnableLog(sessionInv.Log)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("logging to %s\n", path)

	case sessionInv.Nolog != "":
		if err := client.DisableLog(sessionInv.Nolog); err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("file logging disabled")

	case sessionInv.Restart != "":
		if err := client.Restart(sessionInv.Restart); err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("restarted %s\n", sessionInv.Restart)

	default:
		fmt.Println("No session operation specified. Use -status, -log, -nolog, or -restart.")
		os.Exit(1)
	}
}

func handleRun() {
	allTasks, err := taskfile.Load(runInv.Dir, runInv.Task)
	if err != nil {
		fmt.Println("Error loading tasks:")
		fmt.Println(err)
		os.Exit(1)
	}

	if len(runInv.Skip) > 0 {
		for _, id := range runInv.Skip {
			if !allTasks.Has(id) {
				fmt.Printf("Cannot skip %q: task not found.\n", id)
				os.Exit(1)
			}
		}
		skipSet := map[string]struct{}{}
		for _, id := range runInv.Skip {
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

	taskID := runInv.Task
	if !allTasks.Has(taskID) {
		fmt.Printf("Task %q not found.\n", taskID)
		fmt.Println("Run `run -list` for more information about the available tasks.")
		os.Exit(1)
	}

	useTUI := false
	switch runInv.UI {
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
		runErr = tui.Start(ctx, os.Stdin, os.Stdout, runInv.Dir, allTasks, taskID)
	} else {
		subtree := allTasks.Subtree(taskID)
		prn := printer.New(subtree.LongestID(), os.Stdout)
		r, err := runner.New(runner.RunTypeShort, runInv.Dir, allTasks, taskID, prn)
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

// --- Help text helpers ---

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

func versionInline() string {
	b := &strings.Builder{}
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

func licenseInline() string {
	b := &strings.Builder{}
	b.WriteString(indent.String(wordwrap.String(statement, 60), 2) + "\n")
	b.WriteString("\n")
	b.WriteString("  Run `run -license` for more info.\n")
	b.WriteString("\n")
	b.WriteString("  © Andrew Monks <a@monks.co>\n")
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

