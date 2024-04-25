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
	"github.com/amonks/run/internal/styles"
	"github.com/charmbracelet/lipgloss"
)

// Script is a wrapper around exec.Cmd, offering a focused API and robust
// cancelation.
type Script struct {
	Dir  string
	Env  map[string]string
	Text string
}

// New creates a new Script with the given working directory, environment, and
// text. Scripts do nothing until they are started, and can be started many
// times concurrently. If dir is the empty string, the script is run in the
// current working directory. Env is appended to the current environment.
// Script is evaluated in a new bash process. Effectivley, it is equivalent to
//
//	$ cd $DIR $ $ENV bash -c "$TEXT"
func New(dir string, env map[string]string, text string) Script {
	return Script{
		Dir:  dir,
		Env:  env,
		Text: text,
	}
}

// Start executes the script, and does not return until the script is done
// executing. It is safe to call start multiple times, including concurrently.
// The returned error will be nil only if the process exits with status code 0
// and is not interrupted by a context cancelation.
//
// Execution can be canceled with the provided context. When canceled, we first
// send SIGINT, then if the process doesn't exit within 2 seconds, we send
// SIGKILL. Start will always return an error if the context is canceled before
// the script is complete.
func (s Script) Start(ctx context.Context, stdout, stderr io.Writer) error {
	return (&execution{
		script: s,

		cmd:   nil,
		cmdMu: mutex.New("script"),

		stdout: stdout,
		stderr: stderr,
	}).run(ctx)
}

type execution struct {
	script Script

	cmd   *exec.Cmd
	cmdMu *mutex.Mutex

	stdout io.Writer
	stderr io.Writer
}

func (x *execution) run(ctx context.Context) error {
	if err := x.startCmd(); err != nil {
		return err
	}
	defer x.cleanup()

	// Wait for either exit or cancel.
	exit := x.wait()
	select {
	case err := <-exit:
		return err

	case <-ctx.Done():
		x.printf(styles.Log, "canceled; stopping")
	}
	// -- CONTEXT CANCELED ------------------------------------------------
	// Kill the script.

	err := ctx.Err()

	// Do our best to stop the task. First it tries SIGINT, then, if the
	// task is still running after 2 seconds, it tries SIGKILL. Then return
	// any errors encountered along the way.

	errs := []error{err}

	// Never started or already stopped.
	if !x.isRunning() {
		return errors.Join(errs...)
	}

	// Try to SIGINT the pgroup
	if err := x.sigint(); err != nil {
		errs = append(errs, err)
	}

	// Give it 2 seconds to die gracefully after the SIGINT.
	select {
	case <-exit:
		return errors.Join(errs...)
	case <-time.After(2 * time.Second):
	}

	// It's still alive. Resort to SIGKILL.
	if err := x.sigkill(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (x *execution) printf(style lipgloss.Style, f string, args ...interface{}) {
	str := style.Render(fmt.Sprintf(f, args...))
	fmt.Fprintln(x.stderr, str)
}

var findBash sync.Once
var errFindingBash error
var bash = ""

func (x *execution) startCmd() error {
	defer x.cmdMu.Lock("startCmd").Unlock()

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

	var env []string
	for k, v := range x.script.Env {
		env = append(env, fmt.Sprintf(`%s=%s`, k, v))
	}

	x.cmd = exec.Command(bash, "-c", x.script.Text)
	x.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	x.cmd.Dir = x.script.Dir
	x.cmd.Stdout = x.stdout
	x.cmd.Stderr = x.stderr
	x.cmd.Env = append(os.Environ(), env...)

	if err := x.cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (x *execution) wait() <-chan error {
	exit := make(chan error)
	go func() {
		process := x.getProcess()
		if process == nil {
			exit <- nil
		}

		// Wait for process to exit
		if state, err := process.Wait(); err != nil && strings.Contains(err.Error(), "no child processes") {
			exit <- nil
		} else if err != nil {
			exit <- fmt.Errorf("wait err: %w", err)
		} else if code := state.ExitCode(); code != 0 {
			exit <- fmt.Errorf("exit %d", code)
		} else {
			exit <- nil
		}
	}()
	return exit
}

func (x *execution) sigint() error {
	defer x.cmdMu.Lock("sigint").Unlock()

	if x.cmd == nil {
		return nil
	}
	if err := syscall.Kill(-x.cmd.Process.Pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("sigint error: %w", err)
	}
	return nil
}

func (x *execution) sigkill() error {
	defer x.cmdMu.Lock("sigkill").Unlock()

	if x.cmd == nil {
		return nil
	}
	if err := syscall.Kill(-x.cmd.Process.Pid, syscall.SIGKILL); err != nil && !strings.Contains(err.Error(), "no such process") {
		return fmt.Errorf("sigkill error: %w", err)
	}
	return nil
}

func (x *execution) cleanup() {
	defer x.cmdMu.Lock("cleanup").Unlock()

	x.cmd = nil
}

func (x *execution) getProcess() *os.Process {
	defer x.cmdMu.Lock("getProcess").Unlock()
	if x.cmd == nil {
		return nil
	}
	return x.cmd.Process
}

func (x *execution) isRunning() bool {
	defer x.cmdMu.Lock("isRunning").Unlock()

	return x.cmd != nil && x.cmd.ProcessState == nil
}
