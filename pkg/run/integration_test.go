package run_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amonks/run/pkg/run"
	"github.com/muesli/reflow/indent"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func TestIntegrationSnapshots(t *testing.T) {
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
	dmp := diffmatchpatch.New()

	changedFilePath := path.Join("testdata/snapshots", name, "changed-file")
	if f, err := os.Create(changedFilePath); err != nil {
		return err
	} else if err := f.Sync(); err != nil {
		return err
	} else if err := f.Close(); err != nil {
		return err
	}
	defer os.Remove(changedFilePath)

	tasks, err := run.Load(path.Join("testdata/snapshots", name))
	if err != nil {
		return fmt.Errorf("Error loading tasks: %s", err)
	}

	r, err := run.RunTask(path.Join("testdata/snapshots", name), tasks, "test")
	if err != nil {
		return fmt.Errorf("Error running tasks: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ui := newTestUI()

	exit := make(chan error)

	// Start the run.
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
		log = "ok" + "\n\n" + ui.String()
	} else {
		log = exitErr.Error() + "\n\n" + ui.String()
	}

	logfilePath := path.Join("testdata/snapshots", name, "out.log")
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

	if string(expected) != log {
		errFilePath := path.Join("testdata/snapshots", name, "fail.log")
		if err := os.WriteFile(errFilePath, []byte(log), 0644); err != nil {
			return err
		}
		diff := dmp.DiffMain(string(expected), log, false)
		return fmt.Errorf("Unexpected output from example '%s', saved to fail.log:\n%s", name, dmp.DiffPrettyText(diff))
	}

	return nil
}

func newTestUI() *testUI {
	return &testUI{
		bufs: map[string]*strings.Builder{},
	}
}

type testUI struct {
	bufs map[string]*strings.Builder
	mu   sync.Mutex
}

// ui implements run.MultiWriter
var _ run.MultiWriter = &testUI{}

func (ui *testUI) Writer(id string) io.Writer {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	return &testWriter{
		id: id,
		ui: ui,
	}
}

func (ui *testUI) Write(id string, bs []byte) (int, error) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if _, ok := ui.bufs[id]; !ok {
		ui.bufs[id] = &strings.Builder{}
	}
	return ui.bufs[id].Write(bs)
}

func (ui *testUI) String() string {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	var ids []string
	for id := range ui.bufs {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id + ":\n" + indent.String(ui.bufs[id].String(), 2)
	}

	return strings.Join(out, "\n")
}

// testWriter implements io.Writer
var _ io.Writer = &testWriter{}

type testWriter struct {
	mu sync.Mutex
	id string
	ui *testUI
}

func (w *testWriter) Write(bs []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.ui.Write(w.id, bs)
}
