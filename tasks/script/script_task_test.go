// Package script_test covers only the layer in tasks/script which wraps
// internal/script; the script running behavior is tested in internal/script.
package script_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run/internal/safebuffer"
	"github.com/amonks/run/tasks"
	"github.com/amonks/run/tasks/script"
	"github.com/stretchr/testify/assert"
)

func TestCombineStdoutStderr(t *testing.T) {
	var (
		task    = script.New(tasks.TaskMetadata{}, ".", nil, "echo hello ; echo world >&2")
		w       = safebuffer.New()
		onReady = make(chan struct{}, 1)
	)

	err := task.Start(context.Background(), onReady, w)

	assert.NoError(t, err)
	assert.Equal(t, "hello\nworld\n", w.String())
}

func TestOnReady(t *testing.T) {
	t.Run("short task: ready upon completion", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{Type: "short"}, ".", nil, "sleep 1")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
		)

		err := task.Start(context.Background(), onReady, w)
		assert.NoError(t, err)

		_, ok := <-onReady
		assert.False(t, ok)
	})

	t.Run("long task: ready immediately", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{Type: "long"}, ".", nil, "sleep 1")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
		)

		go task.Start(context.Background(), onReady, w)

		select {
		case <-time.After(time.Millisecond * 10):
			t.Fatalf("timeout getting ready")
		case _, ok := <-onReady:
			assert.True(t, ok)
		}
	})
}

func TestEmptyScript(t *testing.T) {
	t.Run("short exits immediately", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{Type: "short"}, ".", nil, "")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
			err     = make(chan error)
		)

		go func() { err <- task.Start(context.Background(), onReady, w) }()

		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatalf("timed out waiting for exit")
		case <-err:
		}
	})

	t.Run("long does not exit until canceled", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{Type: "long"}, ".", nil, "")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
			err     = make(chan error)
		)
		ctx, cancel := context.WithCancel(context.Background())

		go func() { err <- task.Start(ctx, onReady, w) }()

		select {
		case <-err:
			t.Fatalf("exit before cancel")
		case <-time.After(10 * time.Millisecond):
		}

		cancel()

		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatalf("timed out waiting for exit")
		case <-err:
		}
	})
}

func TestDir(t *testing.T) {
	t.Run("dir is normally the cwd", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{}, ".", nil, "pwd")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
		)

		err := task.Start(context.Background(), onReady, w)

		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(strings.TrimSpace(w.String()), "/tasks/script"))
	})

	t.Run("you can override dir", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{}, "/", nil, "pwd")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
		)

		err := task.Start(context.Background(), onReady, w)

		assert.NoError(t, err)
		assert.Equal(t, "/", strings.TrimSpace(w.String()))
	})
}

func TestEnv(t *testing.T) {
	t.Run("values are propagated", func(t *testing.T) {
		var (
			task    = script.New(tasks.TaskMetadata{}, ".", map[string]string{"KEY": "VALUE"}, "echo $KEY")
			w       = safebuffer.New()
			onReady = make(chan struct{}, 1)
		)

		err := task.Start(context.Background(), onReady, w)

		assert.NoError(t, err)
		assert.Equal(t, "VALUE\n", w.String())
	})

	t.Run("existing env is retained", func(t *testing.T) {
		normalEnvSize := getInheritedEnvSize(nil)
		assert.Greater(t, normalEnvSize, 0)
		envSizeWithExtraValue := getInheritedEnvSize(map[string]string{"KEY": "VALUE"})
		assert.Equal(t, envSizeWithExtraValue, normalEnvSize+1)
	})
}

func getInheritedEnvSize(env map[string]string) int {
	var (
		task    = script.New(tasks.TaskMetadata{}, ".", env, "env | wc -l | awk '{print $1}'")
		w       = safebuffer.New()
		onReady = make(chan struct{}, 1)
	)
	task.Start(context.Background(), onReady, w)
	i, err := strconv.ParseInt(strings.TrimSpace(w.String()), 10, 64)
	if err != nil {
		panic(err)
	}
	return int(i)
}
