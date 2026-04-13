package session

import (
	"io"
	"testing"

	tea "charm.land/bubbletea/v2"
	"monks.co/run/runner"
	"monks.co/run/task"
)

func TestClientStatus(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "a", Type: "short"}),
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long", Dependencies: []string{"a"}}),
	)

	r, err := runner.New(runner.RunTypeLong, taskDir, lib, "root", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	send := func(msg tea.Msg) {
		if m, ok := msg.(QueryFileLogMsg); ok {
			m.Reply <- false
		}
	}

	sess, err := newSession("test", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	client, err := connectTo(sock)
	if err != nil {
		t.Fatal(err)
	}

	status, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}

	if status.Session != "test" {
		t.Errorf("session = %q, want %q", status.Session, "test")
	}
	if len(status.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(status.Tasks))
	}
}

func TestClientRestart(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long"}),
	)

	r, err := runner.New(runner.RunTypeLong, taskDir, lib, "root", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	send := func(msg tea.Msg) {
		if m, ok := msg.(QueryFileLogMsg); ok {
			m.Reply <- false
		}
	}

	sess, err := newSession("test", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	client, err := connectTo(sock)
	if err != nil {
		t.Fatal(err)
	}

	// Should not error (task exists, invalidate is fire-and-forget).
	if err := client.Restart("root"); err != nil {
		t.Fatal(err)
	}
}

func TestClientEnableDisableLog(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long"}),
	)

	r, err := runner.New(runner.RunTypeLong, taskDir, lib, "root", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	send := func(msg tea.Msg) {
		switch m := msg.(type) {
		case EnableFileLogMsg:
			m.Reply <- true
		case DisableFileLogMsg:
			m.Reply <- true
		case QueryFileLogMsg:
			m.Reply <- false
		}
	}

	sess, err := newSession("test", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	client, err := connectTo(sock)
	if err != nil {
		t.Fatal(err)
	}

	path, err := client.EnableLog("root")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Error("expected non-empty log path")
	}

	if err := client.DisableLog("root"); err != nil {
		t.Fatal(err)
	}
}

func TestClientConnectNonexistent(t *testing.T) {
	_, err := connectTo("/tmp/nonexistent-run-test.sock")
	if err == nil {
		t.Fatal("expected error connecting to nonexistent socket")
	}
}

// noopMultiWriter is also defined in session_test.go but that's the same
// package, so we can reuse it. If not, define it here:
var _ runner.MultiWriter = &noopMW{}

type noopMW struct{}

func (n *noopMW) Writer(id string) io.Writer { return io.Discard }
