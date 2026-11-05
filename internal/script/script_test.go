package script_test

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"monks.co/run/internal/script"
)

// safeBuffer is a thread-safe bytes.Buffer for use in tests.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestHappyPath(t *testing.T) {
	s := script.Script{Dir: ".", Text: "echo hello"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "hello\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestStdoutAndStderr(t *testing.T) {
	s := script.Script{Dir: ".", Text: "echo out ; echo err >&2"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "out\n", stdout.String())
	assert.Equal(t, "err\n", stderr.String())
}

func TestDir(t *testing.T) {
	s := script.Script{Dir: "/", Text: "pwd"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "/\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestEnv(t *testing.T) {
	s := script.Script{Dir: ".", Env: []string{"FOO=BAR"}, Text: "echo $FOO"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "BAR\n", stdout.String())
}

func TestExitCode(t *testing.T) {
	s := script.Script{Dir: ".", Text: "exit 1"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}

	err := s.Start(context.Background(), stdout, stderr)

	assert.EqualError(t, err, "exit 1")
}

func TestReentrant(t *testing.T) {
	s := script.Script{Dir: ".", Text: "echo hello"}

	for range 3 {
		stdout, stderr := &safeBuffer{}, &safeBuffer{}
		err := s.Start(context.Background(), stdout, stderr)
		assert.NoError(t, err)
		assert.Equal(t, "hello\n", stdout.String())
		assert.Equal(t, "", stderr.String())
	}
}

// waitForOutput waits for the script to print want — the script's own
// word that it has reached a point, in place of guessing how long a
// shell takes to get there.
func waitForOutput(t *testing.T, stdout, stderr *safeBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stdout.String(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("the script never printed %q; stdout %q, stderr %q", want, stdout.String(), stderr.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSIGINT(t *testing.T) {
	s := script.Script{Dir: ".", Text: "echo started ; sleep 100"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)
	go func() { errCh <- s.Start(ctx, stdout, stderr) }()

	// Cancel once the script is running, so the SIGINT reaches it.
	waitForOutput(t, stdout, stderr, "started")
	cancel()

	select {
	case <-time.After(time.Second):
		t.Fatal("script did not exit after SIGINT")
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
	}
}

func TestSIGKILL(t *testing.T) {
	// Trap SIGINT so the process ignores it, forcing SIGKILL.
	s := script.Script{Dir: ".", Text: "trap '' SIGINT ; echo trapped ; sleep 100"}
	stdout, stderr := &safeBuffer{}, &safeBuffer{}
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)
	go func() { errCh <- s.Start(ctx, stdout, stderr) }()

	// Cancel once the trap is set. A SIGINT that lands before bash has
	// run the trap kills it, which would read as the trap not holding.
	waitForOutput(t, stdout, stderr, "trapped")
	cancel()

	// Should NOT exit within the SIGINT grace period.
	select {
	case <-errCh:
		t.Fatal("script exited after SIGINT despite trapping it")
	case <-time.After(time.Second):
	}

	// Should exit after SIGKILL (within another ~1-2 seconds).
	select {
	case <-time.After(3 * time.Second):
		t.Fatal("script did not exit after SIGKILL")
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
	}
}
