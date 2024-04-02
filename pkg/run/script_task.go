package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/amonks/run/internal/mutex"
	"github.com/charmbracelet/lipgloss"
)

// ScriptTask produces a runnable Task from a bash script and working
// directory. The script will execute in metadata.Dir. The script's Stdout and
// Stderr will be provided by the Run, and will be forwarded to the UI. The
// script will not get a Stdin.
//
// Script runs in a new bash process, and can have multiple lines. It is run
// basically like this:
//
//	$ cd $DIR
//	$ bash -c "$CMD" 2&>1 /some/ui
func ScriptTask(script string, dir string, env []string, metadata TaskMetadata) Task {
	return &scriptTask{
		mu:       mutex.New(fmt.Sprintf("script:%s", metadata.ID)),
		dir:      dir,
		script:   script,
		env:      env,
		metadata: metadata,
	}
}

type scriptTask struct {
	mu *mutex.Mutex

	stdout io.Writer

	dir      string
	script   string
	env      []string
	metadata TaskMetadata

	cmd *exec.Cmd
}

// *scriptTask implements Task
var _ Task = &scriptTask{}

func (t *scriptTask) Metadata() TaskMetadata {
	defer t.mu.Lock("Metadata").Unlock()

	meta := t.metadata
	return meta
}

func (t *scriptTask) Start(ctx context.Context, stdout io.Writer) error {
	defer t.cleanup()

	t.stdout = stdout

	if !t.hasScript() {
		// If this is a "long" task, we want to keep running until the
		// run is killed. If this is a "short" task with no script, we
		// should consider it done as soon as its dependencies are.
		if t.Metadata().Type == "long" {
			<-ctx.Done()
		}
		return nil
	}

	// Start the CMD.
	if err := t.startCmd(t.stdout); err != nil {
		return err
	}

	// Handle the CMD's exit.
	exit := make(chan error)
	go func() {
		if process := t.process(); process == nil {
			exit <- nil
		} else if state, err := process.Wait(); err != nil {
			exit <- err
		} else if code := state.ExitCode(); code != 0 {
			exit <- fmt.Errorf("exit %d", code)
		} else {
			exit <- nil
		}
	}()

	select {
	case err := <-exit:
		return err
	case <-ctx.Done():
		t.printf(logStyle, "canceled; stopping")
	}

	err := ctx.Err()

	// Do our best to stop the task. First it tries SIGINT, then, if the task is
	// still running after 2 seconds, it tries SIGKILL. Then return any errors
	// encountered along the way.

	errs := []error{err}

	if !t.hasScript() {
		return errors.Join(errs...)
	}

	// Never started or already stopped.
	if !t.isRunning() {
		return errors.Join(errs...)
	}

	// Try to SIGINT the pgroup
	if err := t.sigint(); err != nil {
		errs = append(errs, err)
	}

	// Give it 2 seconds to die gracefully after the SIGINT.
	select {
	case <-exit:
		return errors.Join(errs...)
	case <-time.After(2 * time.Second):
	}

	// It's still alive. Resort to SIGKILL.
	if err := t.sigkill(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (t *scriptTask) printf(style lipgloss.Style, f string, args ...interface{}) {
	s := style.Render(fmt.Sprintf(f, args...))
	fmt.Fprintln(t.stdout, s)
}

var findBash sync.Once
var errFindingBash error
var bash = ""

func (t *scriptTask) startCmd(stdout io.Writer) error {
	defer t.mu.Lock("startCmd").Unlock()
	if findBash.Do(func() {
		var b bytes.Buffer
		whichBash := exec.Command("/bin/sh", "-c", "which bash")
		whichBash.Stdout = &b
		if errFindingBash = whichBash.Run(); errFindingBash != nil {
			return
		}
		bash = strings.TrimSpace(b.String())
	}); errFindingBash != nil {
		return errFindingBash
	}
	t.cmd = exec.Command(bash, "-c", t.script)
	t.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	t.cmd.Dir = t.dir
	t.cmd.Stdout = stdout
	t.cmd.Stderr = stdout
	t.cmd.Env = append(os.Environ(), t.env...)

	if err := t.cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (t *scriptTask) sigint() error {
	defer t.mu.Lock("sigint").Unlock()
	return syscall.Kill(-t.cmd.Process.Pid, syscall.SIGINT)
}

func (t *scriptTask) sigkill() error {
	defer t.mu.Lock("sigkill").Unlock()
	if err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL); err != nil && !strings.Contains(err.Error(), "no such process") {
		return err
	}
	return nil
}

func (t *scriptTask) cleanup() {
	defer t.mu.Lock("cleanup").Unlock()
	t.cmd = nil
}

func (t *scriptTask) process() *os.Process {
	defer t.mu.Lock("process").Unlock()
	if t.cmd == nil {
		return nil
	}
	return t.cmd.Process
}

func (t *scriptTask) hasScript() bool {
	defer t.mu.Lock("hasScript").Unlock()
	return t.script != ""
}

func (t *scriptTask) isRunning() bool {
	defer t.mu.Lock("isRunning").Unlock()
	return t.cmd != nil && t.cmd.ProcessState == nil
}
