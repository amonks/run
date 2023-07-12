package run_test

import (
	"strings"
	"testing"
	"time"

	"github.com/amonks/run"
)

func TestScriptTaskOK(t *testing.T) {
	task := run.ScriptTask("sleep 0.1; exit 0", ".", run.TaskMetadata{})
	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		if err := task.Start(&b); err != nil {
			t.Fatal(err)
		}

		select {
		case <-task.Wait():
			t.Fatal("task exited unexpectedly")
		default:
		}

		select {
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout")
		case err := <-task.Wait():
			if err != nil {
				t.Fatal("unexpected err")
			}
		}
	}
}

func TestScriptTaskFail(t *testing.T) {
	task := run.ScriptTask("sleep 0.1; exit 1", ".", run.TaskMetadata{})
	for i := 0; i < 3; i++ {
		b := strings.Builder{}

		if err := task.Start(&b); err != nil {
			t.Fatal(err)
		}

		select {
		case <-task.Wait():
			t.Fatal("task exited unexpectedly")
		default:
		}

		select {
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout")
		case err := <-task.Wait():
			if err == nil {
				t.Fatal("expected err")
			}
		}
	}
}
