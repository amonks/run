package run_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run/pkg/run"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func TestIntegrationSnapshots(t *testing.T) {
	t.Skip()

	exs, err := os.ReadDir("testdata/snapshots")
	if err != nil {
		t.Error(err)
	}

	for _, ex := range exs {
		name := ex.Name()
		t.Run(name, func(t *testing.T) {
			if os.Getenv("SKIP_FLAKY_TESTS") == "true" &&
				(name == "long-with-trigger" ||
					name == "three-layers") {

				t.Skip()
			}

			if err := testExample(t, name); err != nil {
				t.Error(err)
			}
		})
	}
}

func testExample(t *testing.T, name string) error {
	// Set up environment variables here so we can validate it's
	// still accessible to the task's command.
	if name == "env" {
		os.Setenv("INHERITED_ENV", "true")
		defer os.Unsetenv("INHERITED_ENV")
	}

	dmp := diffmatchpatch.New()

	changedFilePath := filepath.Join("testdata", "snapshots", name, "changed-file")
	if f, err := os.Create(changedFilePath); err != nil {
		return err
	} else if err := f.Sync(); err != nil {
		return err
	} else if err := f.Close(); err != nil {
		return err
	}
	defer os.Remove(changedFilePath)

	tasks, err := run.Load(filepath.Join("testdata", "snapshots", name))
	if err != nil {
		return fmt.Errorf("Error loading tasks: %s", err)
	}

	r, err := run.RunTask(filepath.Join("testdata", "snapshots", name), tasks, "test")
	if err != nil {
		return fmt.Errorf("Error running tasks: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ui := run.NewPrinter(r)

	exit := make(chan error)

	// Start the run.
	uiReady := make(chan struct{})
	var b strings.Builder
	go ui.Start(ctx, uiReady, nil, &b)
	<-uiReady
	go func() { exit <- r.Start(ctx, ui) }()
	// 1 second into the test, change a file so that tests can exercise
	// file-watching.
	go func() {
		time.Sleep(time.Second)
		os.Remove(changedFilePath)
	}()

	var exitErr error
	if r.Type() == run.RunTypeShort {
		exitErr = <-exit
		cancel()
	} else {
		time.Sleep(5 * time.Second)
		exitErr = errors.New("long")
		cancel()
		<-exit
	}

	var log string
	if exitErr == nil {
		log = "ok" + "\n\n" + b.String()
	} else {
		log = exitErr.Error() + "\n\n" + b.String()
	}

	logfilePath := filepath.Join("testdata", "snapshots", name, "out.log")
	if _, err := os.Stat(logfilePath); os.IsNotExist(err) {
		// Expected output does not exist! Create it.
		if err := os.WriteFile(logfilePath, []byte(log), 0644); err != nil {
			return err
		}
		return nil
	}

	expected, err := os.ReadFile(logfilePath)
	if err != nil {
		return fmt.Errorf("Error reading logfile: %s", err)
	}

	deinterleavedExpectation := deinterleave(string(expected))
	deinterleavedResult := deinterleave(log)

	if deinterleavedResult != deinterleavedExpectation {
		errFilePath := filepath.Join("testdata", "snapshots", name, "fail.log")
		if err := os.WriteFile(errFilePath, []byte(log), 0644); err != nil {
			return err
		}
		diff := dmp.DiffMain(deinterleavedExpectation, deinterleavedResult, false)
		return fmt.Errorf("Unexpected output from example '%s', saved to fail.log:\n%s", name, dmp.DiffPrettyText(diff))
	}

	return nil
}

func deinterleave(interleaved string) string {
	streams := map[string][]string{}
	lines := strings.Split(interleaved, "\n")
	indent := strings.Index(lines[2], "starting")
	if indent == -1 {
		indent = strings.Index(lines[2], "watching")
	}
	if indent == -1 {
		panic("\n" + lines[2] + "\n\n" + lines[0] + "\n" + lines[1] + "\n" + lines[2] + "\n" + lines[3])
	}
	var id string
	for i := 2; i < len(lines); i++ {
		l := lines[i]
		if strings.TrimSpace(l) == "" {
			continue
		}
		possibleID := strings.TrimSpace(l[:indent])
		if possibleID != "" {
			id = possibleID
		}
		streams[id] = append(streams[id], l[indent:])
	}

	var ids []string
	for id := range streams {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := lines[0] + "\n"
	for _, id := range ids {
		out += id
		for _, line := range streams[id] {
			out += "  " + line + "\n"
		}
		out += "\n"
	}
	return out
}
