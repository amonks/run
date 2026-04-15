package session

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"monks.co/run/runner"
	"monks.co/run/task"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"TaskStatusRunning", "running"},
		{"TaskStatusNotStarted", "not_started"},
		{"TaskStatusFailed", "failed"},
		{"TaskStatusDone", "done"},
		{"TaskStatusRestarting", "restarting"},
		{"TaskStatusCanceled", "canceled"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := statusString(tt.in)
			if got != tt.want {
				t.Errorf("statusString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSessionGetTasks(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	// Create a minimal run with a couple of tasks.
	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "a", Type: "short"}),
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long", Dependencies: []string{"a"}}),
	)

	r, err := runner.New(runner.RunTypeLong, taskDir, lib, "root", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	// A send function that handles QueryFileLogMsg (always returns false).
	send := func(msg tea.Msg) {
		if m, ok := msg.(QueryFileLogMsg); ok {
			m.Reply <- false
		}
	}

	sess, err := newSession("test-session", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	// Make an HTTP request to the socket.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: unixDialer(sess.sock),
		},
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get("http://localhost/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Session string `json:"session"`
		Tasks   []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Log    bool   `json:"log"`
		} `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body.Session != "test-session" {
		t.Errorf("session = %q, want %q", body.Session, "test-session")
	}
	if len(body.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(body.Tasks))
	}
	// Tasks should be "a" and "root" (internal @-prefixed tasks are filtered).
	ids := map[string]bool{}
	for _, task := range body.Tasks {
		ids[task.ID] = true
	}
	if !ids["a"] || !ids["root"] {
		t.Errorf("expected tasks 'a' and 'root', got %v", ids)
	}
}

func TestSessionRestart(t *testing.T) {
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

	sess, err := newSession("test-session", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: unixDialer(sess.sock),
		},
		Timeout: 2 * time.Second,
	}

	resp, err := client.Post("http://localhost/restart/root", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["ok"] != true {
		t.Errorf("expected ok=true, got %v", body)
	}
}

func TestSessionRestartSlashID(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "apps/air/build", Type: "short"}),
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long", Dependencies: []string{"apps/air/build"}}),
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

	sess, err := newSession("test-session", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	c, err := connectTo(sock)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Restart("apps/air/build"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
}

func TestSessionRestartNotFound(t *testing.T) {
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

	sess, err := newSession("test-session", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	c, err := connectTo(sock)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Restart("does/not/exist")
	if err == nil {
		t.Fatal("expected error for unknown task, got nil")
	}
}

func TestSessionEnableDisableLog(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long"}),
	)

	r, err := runner.New(runner.RunTypeLong, taskDir, lib, "root", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	// Track what messages are sent.
	var lastMsg tea.Msg
	send := func(msg tea.Msg) {
		lastMsg = msg
		switch m := msg.(type) {
		case EnableFileLogMsg:
			m.Reply <- true
		case DisableFileLogMsg:
			m.Reply <- true
		case QueryFileLogMsg:
			m.Reply <- false
		}
	}

	sess, err := newSession("test-session", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: unixDialer(sess.sock),
		},
		Timeout: 2 * time.Second,
	}

	// Enable log.
	resp, err := client.Post("http://localhost/log/root", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("enable log: status = %d, want 200", resp.StatusCode)
	}
	if _, ok := lastMsg.(EnableFileLogMsg); !ok {
		t.Errorf("expected EnableFileLogMsg, got %T", lastMsg)
	}

	// Disable log.
	req, _ := http.NewRequest("DELETE", "http://localhost/log/root", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("disable log: status = %d, want 200", resp.StatusCode)
	}
	if _, ok := lastMsg.(DisableFileLogMsg); !ok {
		t.Errorf("expected DisableFileLogMsg, got %T", lastMsg)
	}
}

func TestSessionLogSlashID(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "apps/air/build", Type: "short"}),
		task.FuncTask(nil, task.TaskMetadata{ID: "root", Type: "long", Dependencies: []string{"apps/air/build"}}),
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

	sess, err := newSession("test-session", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	c, err := connectTo(sock)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.EnableLog("apps/air/build"); err != nil {
		t.Fatalf("EnableLog: %v", err)
	}
	if err := c.DisableLog("apps/air/build"); err != nil {
		t.Fatalf("DisableLog: %v", err)
	}
}

func TestStaleSocketCleanup(t *testing.T) {
	sock := tempSock(t)

	// Create a stale socket file (not actually listening).
	os.MkdirAll(filepath.Dir(sock), 0700)
	os.WriteFile(sock, []byte("stale"), 0600)

	taskDir := t.TempDir()
	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "stale", Type: "long"}),
	)

	r, err := runner.New(runner.RunTypeLong, taskDir, lib, "stale", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	send := func(msg tea.Msg) {}
	sess, err := newSession("stale", t.TempDir(), sock, r, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	// Session should have replaced the stale socket.
	if _, err := os.Stat(sock); err != nil {
		t.Errorf("socket should exist after replacing stale one: %v", err)
	}
}

func TestDuplicateSessionFails(t *testing.T) {
	sock := tempSock(t)
	taskDir := t.TempDir()

	lib := task.NewLibrary(
		task.FuncTask(nil, task.TaskMetadata{ID: "dup", Type: "long"}),
	)

	r1, err := runner.New(runner.RunTypeLong, taskDir, lib, "dup", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	send := func(msg tea.Msg) {}
	sess1, err := newSession("dup", t.TempDir(), sock, r1, send)
	if err != nil {
		t.Fatal(err)
	}
	defer sess1.Close()

	r2, err := runner.New(runner.RunTypeLong, taskDir, lib, "dup", &noopMultiWriter{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = newSession("dup", t.TempDir(), sock, r2, send)
	if err == nil {
		t.Fatal("expected error for duplicate session")
	}
	if got := err.Error(); got != "session 'dup' is already running in this directory" {
		t.Errorf("unexpected error: %s", got)
	}
}

// --- helpers ---

type noopMultiWriter struct{}

func (n *noopMultiWriter) Writer(id string) io.Writer {
	return io.Discard
}

func unixDialer(sock string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", sock)
	}
}

// tempSock returns a short socket path suitable for Unix domain sockets
// (which have a ~104 char limit on macOS).
func tempSock(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "run-test-*.sock")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()
	os.Remove(path)
	t.Cleanup(func() { os.Remove(path) })
	return path
}
