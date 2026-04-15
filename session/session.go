package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"monks.co/run/runner"
)

// Session serves an HTTP API on a Unix domain socket for programmatic
// access to a running run session.
type Session struct {
	name   string
	dir    string
	sock   string
	ln     net.Listener
	server *http.Server
	run    *runner.Run
	send   func(tea.Msg)
}

// New creates a session, binding to a Unix domain socket. It starts the
// HTTP server in a background goroutine.
//
// The send function dispatches messages into the BubbleTea event loop
// for operations that touch TUI model state (file logging).
func New(name string, dir string, run *runner.Run, send func(tea.Msg)) (*Session, error) {
	return newSession(name, dir, SocketPath(name, dir), run, send)
}

func newSession(name string, dir string, sock string, run *runner.Run, send func(tea.Msg)) (*Session, error) {

	// Check for existing socket.
	if err := checkStaleSocket(name, sock); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(sock), 0700); err != nil {
		return nil, fmt.Errorf("session: create state dir: %w", err)
	}

	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("session: listen: %w", err)
	}

	s := &Session{
		name: name,
		dir:  dir,
		sock: sock,
		ln:   ln,
		run:  run,
		send: send,
	}

	// Task IDs contain slashes (e.g. "apps/air/build"), so action-first
	// routes with a trailing {id...} wildcard are used rather than
	// "/tasks/{id}/action" — Go's ServeMux {id} matches a single segment.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", s.handleGetTasks)
	mux.HandleFunc("POST /log/{id...}", s.handleEnableLog)
	mux.HandleFunc("DELETE /log/{id...}", s.handleDisableLog)
	mux.HandleFunc("POST /restart/{id...}", s.handleRestart)

	s.server = &http.Server{Handler: mux}
	go s.server.Serve(ln)

	return s, nil
}

// Close shuts down the HTTP server and removes the socket file.
func (s *Session) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.server.Shutdown(ctx)
	return os.Remove(s.sock)
}

func (s *Session) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	ids := s.run.IDs()
	type taskInfo struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Log    bool   `json:"log"`
	}
	tasks := make([]taskInfo, 0, len(ids))
	for _, id := range ids {
		if strings.HasPrefix(id, "@") {
			continue
		}
		logEnabled := s.queryFileLog(id)
		tasks = append(tasks, taskInfo{
			ID:     id,
			Status: statusString(s.run.TaskStatus(id).String()),
			Log:    logEnabled,
		})
	}
	writeJSON(w, map[string]any{
		"session": s.name,
		"tasks":   tasks,
	})
}

func (s *Session) handleEnableLog(w http.ResponseWriter, r *http.Request) {
	id := decodeTaskID(r.PathValue("id"))
	path := LogFilePath(s.name, s.dir, id)
	reply := make(chan bool, 1)
	s.send(EnableFileLogMsg{TaskID: id, Path: path, Reply: reply})
	ok := <-reply
	if !ok {
		http.Error(w, fmt.Sprintf("task %q not found", id), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id, "path": path})
}

func (s *Session) handleDisableLog(w http.ResponseWriter, r *http.Request) {
	id := decodeTaskID(r.PathValue("id"))
	reply := make(chan bool, 1)
	s.send(DisableFileLogMsg{TaskID: id, Reply: reply})
	<-reply
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Session) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := decodeTaskID(r.PathValue("id"))
	if !s.run.Tasks().Has(id) {
		http.Error(w, fmt.Sprintf("task %q not found", id), http.StatusNotFound)
		return
	}
	s.run.Invalidate(id)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Session) queryFileLog(id string) bool {
	reply := make(chan bool, 1)
	s.send(QueryFileLogMsg{TaskID: id, Reply: reply})
	return <-reply
}

// EnableFileLogMsg, DisableFileLogMsg, and QueryFileLogMsg are re-exported
// from the tui package to avoid import cycles. The session package defines
// the canonical types; the tui package handles them.

// EnableFileLogMsg asks the TUI to enable file logging for a task.
type EnableFileLogMsg struct {
	TaskID string
	Path   string
	Reply  chan<- bool
}

// DisableFileLogMsg asks the TUI to disable file logging for a task.
type DisableFileLogMsg struct {
	TaskID string
	Reply  chan<- bool
}

// QueryFileLogMsg asks the TUI whether file logging is enabled for a task.
type QueryFileLogMsg struct {
	TaskID string
	Reply  chan<- bool
}

func decodeTaskID(raw string) string {
	// Path values from net/http already handle percent-encoding.
	// Also support dash-separated slugs for convenience.
	return raw
}

func statusString(str string) string {
	// runner.TaskStatus.String() returns e.g. "TaskStatusRunning".
	// Convert to the API form: "running".
	str = strings.TrimPrefix(str, "TaskStatus")
	// Convert CamelCase to snake_case.
	var b strings.Builder
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func checkStaleSocket(name string, sock string) error {
	// If no socket exists, we're good.
	if _, err := os.Stat(sock); os.IsNotExist(err) {
		return nil
	}

	// Try to connect. If refused, it's stale — remove it.
	conn, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		os.Remove(sock)
		return nil
	}
	conn.Close()

	// Socket is live — another instance is running.
	return fmt.Errorf("session '%s' is already running in this directory", name)
}
