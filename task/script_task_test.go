package task_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run/task"
)

func TestScriptTaskOK(t *testing.T) {
	tk := task.ScriptTask("sleep 0.1; exit 0", ".", nil, task.TaskMetadata{})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		exit := make(chan error)
		go func() { exit <- tk.Start(ctx, make(chan struct{}, 1), &b) }()

		select {
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout")
		case err := <-exit:
			if err != nil {
				t.Fatal("unexpected err")
			}
		}
	}
}

func TestScriptTaskFail(t *testing.T) {
	tk := task.ScriptTask("sleep 0.1; exit 1", ".", nil, task.TaskMetadata{})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		exit := make(chan error)
		go func() { exit <- tk.Start(ctx, make(chan struct{}, 1), &b) }()

		select {
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout")
		case err := <-exit:
			if err == nil {
				t.Fatal("expected success")
			}
		}
	}
}
