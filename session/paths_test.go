package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirSlug(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{
			name: "absolute path",
			dir:  "/Users/ajm/git/amonks/monks.co",
			want: "Users-ajm-git-amonks-monks.co",
		},
		{
			name: "root",
			dir:  "/",
			want: "",
		},
		{
			name: "single component",
			dir:  "/tmp",
			want: "tmp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dirSlug(tt.dir)
			if got != tt.want {
				t.Errorf("dirSlug(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestTaskSlug(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		{"apps/ping/dev", "apps-ping-dev"},
		{"build", "build"},
		{"apps/calendar/dev", "apps-calendar-dev"},
	}
	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			got := taskSlug(tt.taskID)
			if got != tt.want {
				t.Errorf("taskSlug(%q) = %q, want %q", tt.taskID, got, tt.want)
			}
		})
	}
}

func TestSocketPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got := SocketPath("brigid", "/Users/ajm/git/amonks/monks.co")
	want := filepath.Join(home, ".local", "state", "run", "Users-ajm-git-amonks-monks.co", "brigid.sock")
	if got != want {
		t.Errorf("SocketPath = %q, want %q", got, want)
	}
}

func TestLogFilePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got := LogFilePath("brigid", "/Users/ajm/git/amonks/monks.co", "apps/ping/dev")
	want := filepath.Join(home, ".local", "share", "run", "Users-ajm-git-amonks-monks.co", "brigid", "logs", "apps-ping-dev.log")
	if got != want {
		t.Errorf("LogFilePath = %q, want %q", got, want)
	}
}
