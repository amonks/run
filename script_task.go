package run

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ScriptTask produces a runnable Task from a bash script and working
// directory. The script will execute in metadata.Dir. The script's Stdout and
// Stderr will be provided by the Run, and will be forwarded to the UI. The
// script will not get a Stdin.
//
// Script runs in a new bash process, and can have multiple lines. It is run
// basically like this:
//     $ cd dir
//     $ bash -c "$CMD" 2&>1 /some/ui
func ScriptTask(script string, dir string, metadata TaskMetadata) Task {
	return &scriptTask{
		mu:       newMutex(fmt.Sprintf("script")),
		dir:      dir,
		script:   script,
		metadata: metadata,
	}
}

type scriptTask struct {
	mu *mutex

	dir      string
	script   string
	metadata TaskMetadata

	cmd     *exec.Cmd
	waiters []chan<- error
}

// *scriptTask implements Task
var _ Task = &scriptTask{}

func (t *scriptTask) Metadata() TaskMetadata {
	defer t.mu.Lock("Metadata").Unlock()

	meta := t.metadata
	return meta
}

func (t *scriptTask) Start(stdout io.Writer) error {
	t.mu.printf("Start")
	if err := t.Stop(); err != nil {
		t.mu.printf("Start: error stopping")
		return err
	}

	if !t.hasScript() {
		t.mu.printf("Start: no script")
		return nil
	}

	// Start the CMD.
	if err := t.startCmd(stdout); err != nil {
		t.mu.printf("Start: error starting")
		return err
	}

	// Handle the CMD's exit.
	go func() {
		t.mu.printf("Start: waiting")
		if process := t.process(); process == nil {
			return
		} else if state, err := process.Wait(); err != nil {
			t.mu.printf("Start: wait err")
			t.notify(err)
		} else if code := state.ExitCode(); code != 0 {
			t.mu.printf("Start: exit !0")
			t.notify(fmt.Errorf("exit %d", code))
		} else {
			t.mu.printf("Start: exit =0")
			t.notify(nil)
		}
	}()

	return nil
}

func (t *scriptTask) Wait() <-chan error {
	defer t.mu.Lock("Wait").Unlock()
	c := make(chan error)
	// close immediately if not running
	if t.script != "" && (t.cmd == nil || t.cmd.ProcessState != nil) {
		close(c)
		return c
	}
	t.waiters = append(t.waiters, c)
	return c
}

// Stop does its best to stop the task. First it tries SIGINT, then, if the
// task is still running after 2 seconds, it tries SIGKILL. Then it returns any
// errors it encountered along the way.
func (t *scriptTask) Stop() error {
	t.mu.printf("Stop")
	defer t.cleanup()

	var errs []error

	if !t.hasScript() {
		t.mu.printf("Stop: no script")
		return errors.Join(errs...)
	}

	// Never started or already stopped.
	if !t.isRunning() {
		t.mu.printf("Stop: not running")
		return errors.Join(errs...)
	}

	// Try to SIGINT the pgroup
	if err := t.sigint(); err != nil {
		errs = append(errs, err)
	}

	// Give it 2 seconds to die gracefully after the SIGINT.
	select {
	case <-t.Wait():
		t.mu.printf("Stop: sigint worked")
		return errors.Join(errs...)
	case <-time.After(2 * time.Second):
		t.mu.printf("Stop: timeout")
	}

	// It's still alive. Resort to SIGKILL.
	if err := t.sigkill(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (t *scriptTask) startCmd(stdout io.Writer) error {
	defer t.mu.Lock("startCmd").Unlock()
	t.cmd = exec.Command("/bin/bash", "-c", t.script)
	t.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	t.cmd.Dir = t.dir
	t.cmd.Stdout = stdout
	t.cmd.Stderr = stdout
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
		t.mu.printf("Stop: sigkill error %s", err)
		return err
	}
	return nil
}

func (t *scriptTask) cleanup() {
	defer t.mu.Lock("cleanup").Unlock()
	t.cmd = nil
	t.waiters = nil
}

func (t *scriptTask) notify(err error) {
	defer t.mu.Lock("notify").Unlock()
	for _, w := range t.waiters {
		select {
		case w <- err:
		default:
		}
		close(w)
	}
	t.waiters = nil
	t.cmd = nil
}

func (t *scriptTask) process() *os.Process {
	defer t.mu.Lock("process").Unlock()
	if t.cmd == nil {
		return nil
	}
	return t.cmd.Process
}

func (t *scriptTask) processState() *os.ProcessState {
	defer t.mu.Lock("processState").Unlock()
	if t.cmd == nil {
		return nil
	}
	return t.cmd.ProcessState
}

func (t *scriptTask) hasScript() bool {
	defer t.mu.Lock("hasScript").Unlock()
	return t.script != ""
}

func (t *scriptTask) isRunning() bool {
	defer t.mu.Lock("isRunning").Unlock()
	return t.cmd != nil && t.cmd.ProcessState != nil
}
