package script_test

import (
	"context"
	"testing"
	"time"

	"github.com/amonks/run/internal/safebuffer"
	"github.com/amonks/run/internal/script"
	"github.com/stretchr/testify/assert"
)

func TestHappyPath(t *testing.T) {
	s := script.New(".", nil, "echo hello\n>&2 echo world")
	stdout, stderr := safebuffer.New(), safebuffer.New()

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "hello\n", stdout.String())
	assert.Equal(t, "world\n", stderr.String())
}

func TestDir(t *testing.T) {
	s := script.New("/", nil, "pwd")
	stdout, stderr := safebuffer.New(), safebuffer.New()

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "/\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestEnv(t *testing.T) {
	s := script.New(".", map[string]string{"FOO": "BAR"}, "echo $FOO\n>&2 echo $PATH")
	stdout, stderr := safebuffer.New(), safebuffer.New()

	err := s.Start(context.Background(), stdout, stderr)

	assert.NoError(t, err)
	assert.Equal(t, "BAR\n", stdout.String())
	assert.Greater(t, len(stderr.String()), 10)
}

func TestExitCode(t *testing.T) {
	s := script.New(".", nil, "exit 1")
	stdout, stderr := safebuffer.New(), safebuffer.New()

	err := s.Start(context.Background(), stdout, stderr)

	assert.EqualError(t, err, "exit 1")
	assert.Equal(t, "", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSIGINT(t *testing.T) {
	s := script.New(".", nil, "sleep 100")
	stdout, stderr := safebuffer.New(), safebuffer.New()
	ctx, cancel := context.WithCancel(context.Background())

	err := make(chan error)
	go func() { err <- s.Start(ctx, stdout, stderr) }()

	// wait enough time for the script to start
	time.Sleep(10 * time.Millisecond)

	cancel()

	select {
	case <-time.After(time.Second):
		t.Fatalf("script did not exit after sigint")
	case err := <-err:
		assert.EqualError(t, err, "context canceled")
		assert.Equal(t, "", stdout.String())
		assert.Equal(t, "canceled; stopping\n", stderr.String())
	}
}

func TestSIGKILL(t *testing.T) {
	s := script.New(".", nil, "trap '' SIGINT ; sleep 100 ; echo done")
	stdout, stderr := safebuffer.New(), safebuffer.New()
	ctx, cancel := context.WithCancel(context.Background())

	err := make(chan error)
	go func() { err <- s.Start(ctx, stdout, stderr) }()

	// wait enough time for the script to start
	time.Sleep(10 * time.Millisecond)

	cancel()
	select {
	case <-err:
		t.Fatalf("script exited after SIGINT")
	case <-time.After(time.Second):
	}

	select {
	case <-time.After(2 * time.Second):
		t.Fatalf("script did not exit after SIGKILL")
	case err := <-err:
		assert.EqualError(t, err, "context canceled")
		assert.Equal(t, "", stdout.String())
		assert.Equal(t, "canceled; stopping\n", stderr.String())
	}
}
