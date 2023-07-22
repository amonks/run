package run_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/amonks/run/pkg/run"
)

func TestScriptTaskOK(t *testing.T) {
	task := run.ScriptTask("sleep 0.1; exit 0", ".", nil, run.TaskMetadata{})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		exit := make(chan error)
		go func() { exit <- task.Start(ctx, &b) }()

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
	task := run.ScriptTask("sleep 0.1; exit 1", ".", nil, run.TaskMetadata{})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		exit := make(chan error)
		go func() { exit <- task.Start(ctx, &b) }()

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
