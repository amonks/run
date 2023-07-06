package runner_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amonks/runner"
	"github.com/muesli/reflow/indent"
)

func TestIntegrationExamples(t *testing.T) {
	exs, err := ioutil.ReadDir("testdata/examples")
	if err != nil {
		t.Error(err)
	}

	for _, ex := range exs {
		name := ex.Name()
		t.Run(name, func(t *testing.T) {
			if err := testExample(t, name); err != nil {
				t.Error(err)
			}
		})
	}
}

func testExample(t *testing.T, name string) error {
	changedFilePath := path.Join("testdata/examples", name, "changed-file")
	if f, err := os.Create(changedFilePath); err != nil {
		return err
	} else if err := f.Sync(); err != nil {
		return err
	} else if err := f.Close(); err != nil {
		return err
	}

	defer func() {
		os.Remove(changedFilePath)
	}()

	tasks, err := runner.Load(path.Join("testdata/examples", name))
	if err != nil {
		return fmt.Errorf("Error loading tasks: %s", err)
	}

	run, err := runner.RunTask(path.Join("testdata/examples", name), tasks, "test")
	if err != nil {
		return fmt.Errorf("Error running tasks: %s", err)
	}

	ui := newTestUI()
	run.Start(ui)

	go func() {
		time.Sleep(time.Second)
		if err := os.Remove(changedFilePath); err != nil {
			panic(err)
		}
	}()

	var exit error
	if run.Type() == runner.RunTypeShort {
		exit = <-run.Wait()
	} else {
		time.Sleep(5 * time.Second)
		exit = errors.New("long")
		run.Stop()
	}

	var log string
	if exit == nil {
		log = "ok" + "\n\n" + ui.String()
	} else {
		log = exit.Error() + "\n\n" + ui.String()
	}

	logfilePath := path.Join("testdata/examples", name, "out.log")
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
		errFilePath := path.Join("testdata/examples", name, "fail.log")
		if err := os.WriteFile(errFilePath, []byte(log), 0644); err != nil {
			return err
		}
		return fmt.Errorf("Unexpected output from example '%s', saved to fail.log", name)
	}

	run.Stop()

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

// ui implements runner.MultiWriter
var _ runner.MultiWriter = &testUI{}

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
