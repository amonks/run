package runner

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ScriptTask produces a runnable Task from a bash script and working directory.
// multiple lines. The script will execute in metadata.Dir. The script's Stdout
// and Stderr will be provided by the Run, and will be forwarded to the UI. The
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
	stdout  io.Writer
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
	_ = t.Stop()
	defer t.mu.Lock("Start").Unlock()

	t.cmd = nil
	t.waiters = nil

	if t.script == "" {
		return nil
	}

	t.cmd = exec.Command("/bin/bash", "-c", t.script)
	t.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	t.cmd.Dir = t.dir
	t.cmd.Stdout = stdout
	t.cmd.Stderr = stdout

	// Start the CMD.
	if err := t.cmd.Start(); err != nil {
		return err
	}

	// Handle the CMD's exit.
	go func() {
		// already exited
		if t.cmd.ProcessState != nil {
			if code := t.cmd.ProcessState.ExitCode(); code != 0 {
				t.notify(fmt.Errorf("exit %d", code))
			} else {
				t.notify(nil)
			}
			return
		}

		if state, err := t.cmd.Process.Wait(); err != nil {
			t.notify(err)
		} else if code := state.ExitCode(); code != 0 {
			t.notify(fmt.Errorf("exit %d", code))
		} else {
			t.notify(nil)
		}
	}()

	return nil
}

func (t *scriptTask) Wait() <-chan error {
	defer t.mu.Lock("Wait").Unlock()

	c := make(chan error)
	t.waiters = append(t.waiters, c)
	return c
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
}

func (t *scriptTask) Stop() error {
	waitC := t.Wait()
	t.mu.Lock("Stop")

	if t.script == "" {
		t.mu.Unlock()
		t.notify(nil)
		return nil
	}

	if t.cmd == nil || t.cmd.ProcessState != nil {
		// never started or already stopped
		t.mu.Unlock()
		return nil
	}

	// Try to SIGINT the pgroup
	t.mu.printf("Stop: will sigint\n")
	if err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.mu.printf("Stop: sigint worked\n")
		t.mu.Unlock()
		return err
	}
	t.mu.printf("Stop: waiting\n")
	t.mu.Unlock()

	// Give it 2 seconds to die gracefully after the SIGINT.
	select {
	case <-time.After(2 * time.Second):
		t.mu.printf("Stop: timeout\n")
	case err := <-waitC:
		t.mu.printf("Stop: sigint worked\n")
		return err
	}

	defer t.mu.Lock("stop 2").Unlock()

	// It's still alive. Resort to SIGKILL.
	t.mu.printf("Stop: trying sigkill\n")
	if err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL); err != nil && !strings.Contains(err.Error(), "no such process") {
		t.mu.printf("Stop: sigkill error %s\n", err)
		return err
	}
	t.mu.printf("Stop: did sigkill\n")

	return nil
}
