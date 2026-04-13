package session

import (
	"os"
	"path/filepath"
	"strings"
)

// SocketPath returns the path to the Unix domain socket for a session.
// The session is identified by its root task name and the directory where
// run was invoked.
func SocketPath(name string, dir string) string {
	return filepath.Join(stateDir(), dirSlug(dir), name+".sock")
}

// LogFilePath returns the path to the log file for a task within a session.
// Paths are deterministic so clients can compute them without querying the
// session.
func LogFilePath(sessionName string, dir string, taskID string) string {
	return filepath.Join(dataDir(), dirSlug(dir), sessionName, "logs", taskSlug(taskID)+".log")
}

func dirSlug(dir string) string {
	dir = normalizePath(dir)
	dir = strings.TrimPrefix(dir, "/")
	return strings.ReplaceAll(dir, string(filepath.Separator), "-")
}

func taskSlug(taskID string) string {
	return strings.ReplaceAll(taskID, "/", "-")
}

func stateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("session: cannot determine home directory: " + err.Error())
	}
	return filepath.Join(home, ".local", "state", "run")
}

func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("session: cannot determine home directory: " + err.Error())
	}
	return filepath.Join(home, ".local", "share", "run")
}

// normalizePath strips macOS /private prefixes when the trimmed path refers
// to the same location.
func normalizePath(path string) string {
	trimmed := strings.TrimPrefix(path, "/private")
	if trimmed == path {
		return path
	}
	originalInfo, err := os.Stat(path)
	if err != nil {
		return path
	}
	trimmedInfo, err := os.Stat(trimmed)
	if err != nil {
		return path
	}
	if os.SameFile(originalInfo, trimmedInfo) {
		return trimmed
	}
	return path
}
