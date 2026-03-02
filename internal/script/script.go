// Package script provides an immutable, reentrant wrapper around exec.Cmd for
// running bash scripts with robust cancellation.
//
// A Script is a value type describing what to run. Each call to Start creates
// a fresh process; multiple Starts can run concurrently on the same Script.
//
// On context cancellation, Start sends SIGINT to the process group, waits up
// to 2 seconds for a graceful exit, then sends SIGKILL.
package script

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
)

// Script describes a bash script to execute. It is an immutable value type:
// safe to copy, compare, and start multiple times concurrently.
type Script struct {
	Dir  string
	Env  []string
	Text string
}

// Start executes the script in a new bash process and blocks until it
// completes or the context is canceled. It is safe to call Start multiple
// times, including concurrently.
//
// stdout and stderr receive the script's respective output streams.
//
// On context cancellation, Start sends SIGINT to the process group, waits up
// to 2 seconds, then sends SIGKILL. The returned error always includes
// context.Canceled when the context is canceled before the script completes.
func (s Script) Start(ctx context.Context, stdout, stderr io.Writer) error {
	return (&execution{
		script: s,
		mu:     mutex.New("script"),
		stdout: stdout,
		stderr: stderr,
	}).run(ctx)
}

type execution struct {
	script Script
	mu     *mutex.Mutex
	cmd    *exec.Cmd
	stdout io.Writer
	stderr io.Writer
}

func (x *execution) run(ctx context.Context) error {
	if err := x.startCmd(); err != nil {
		return err
	}
	defer x.cleanup()

	exit := x.wait()
	select {
	case err := <-exit:
		return err
	case <-ctx.Done():
	}

	// Context canceled — kill the script.
	err := ctx.Err()
	errs := []error{err}

	if !x.isRunning() {
		return errors.Join(errs...)
	}

	// Try SIGINT first.
	if err := x.sigint(); err != nil {
		errs = append(errs, err)
	}

	// Give it 2 seconds to die gracefully.
	select {
	case <-exit:
		return errors.Join(errs...)
	case <-time.After(2 * time.Second):
	}

	// Resort to SIGKILL.
	if err := x.sigkill(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

var findBash sync.Once
var errFindingBash error
var bash = ""

func (x *execution) startCmd() error {
	defer x.mu.Lock("startCmd").Unlock()

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

	x.cmd = exec.Command(bash, "-c", x.script.Text)
	x.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	x.cmd.Dir = x.script.Dir
	x.cmd.Stdout = x.stdout
	x.cmd.Stderr = x.stderr
	x.cmd.Env = append(os.Environ(), x.script.Env...)

	return x.cmd.Start()
}

func (x *execution) wait() <-chan error {
	exit := make(chan error, 1)
	go func() {
		cmd := x.getCmd()
		if cmd == nil {
			exit <- nil
			return
		}
		// Use cmd.Wait() rather than process.Wait() so that we block
		// until the I/O copying goroutines for stdout/stderr have
		// finished, not just until the process exits.
		err := cmd.Wait()
		if err == nil {
			exit <- nil
		} else if strings.Contains(err.Error(), "no child processes") {
			exit <- nil
		} else {
			exit <- fmt.Errorf("exit %d", cmd.ProcessState.ExitCode())
		}
	}()
	return exit
}

func (x *execution) sigint() error {
	defer x.mu.Lock("sigint").Unlock()
	if x.cmd == nil {
		return nil
	}
	if err := syscall.Kill(-x.cmd.Process.Pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("sigint error: %w", err)
	}
	return nil
}

func (x *execution) sigkill() error {
	defer x.mu.Lock("sigkill").Unlock()
	if x.cmd == nil {
		return nil
	}
	if err := syscall.Kill(-x.cmd.Process.Pid, syscall.SIGKILL); err != nil && !strings.Contains(err.Error(), "no such process") {
		return fmt.Errorf("sigkill error: %w", err)
	}
	return nil
}

func (x *execution) cleanup() {
	defer x.mu.Lock("cleanup").Unlock()
	x.cmd = nil
}

func (x *execution) getCmd() *exec.Cmd {
	defer x.mu.Lock("getCmd").Unlock()
	return x.cmd
}

func (x *execution) isRunning() bool {
	defer x.mu.Lock("isRunning").Unlock()
	return x.cmd != nil && x.cmd.ProcessState == nil
}
