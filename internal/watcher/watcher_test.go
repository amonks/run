package watcher_test

import (
	"testing"
	"time"

	"github.com/amonks/run/internal/watcher"
)

func TestSplitNoGlob(t *testing.T) {
	path, g := watcher.Split("src/website")
	if path != "src/website" {
		t.Errorf("expected 'src/website', got %q", path)
	}
	if g != nil {
		t.Error("expected nil glob for path without wildcards")
	}
}

func TestSplitWithGlob(t *testing.T) {
	path, g := watcher.Split("src/website/**/*.js")
	if path != "src/website/..." {
		t.Errorf("expected 'src/website/...', got %q", path)
	}
	if g == nil {
		t.Fatal("expected non-nil glob")
	}
	if !g.Match("src/website/foo/bar.js") {
		t.Error("glob should match src/website/foo/bar.js")
	}
	if g.Match("src/other/bar.js") {
		t.Error("glob should not match src/other/bar.js")
	}
}

func TestSplitDot(t *testing.T) {
	path, g := watcher.Split(".")
	if path != "." {
		t.Errorf("expected '.', got %q", path)
	}
	if g != nil {
		t.Error("expected nil glob for '.'")
	}
}

func TestDebounce(t *testing.T) {
	in := make(chan watcher.EventInfo, 10)

	out := watcher.Debounce(50*time.Millisecond, in)

	// Send several events quickly.
	in <- watcher.EventInfo{Path: "a.txt", Event: "Write"}
	in <- watcher.EventInfo{Path: "b.txt", Event: "Write"}
	in <- watcher.EventInfo{Path: "c.txt", Event: "Create"}

	select {
	case batch := <-out:
		if len(batch) != 3 {
			t.Errorf("expected 3 events, got %d", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for debounced events")
	}
}

func TestMockAndDispatch(t *testing.T) {
	restore := watcher.Mock()
	defer restore()

	ch, stop, err := watcher.Watch("src/**/*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	go func() {
		watcher.Dispatch("src/**/*.go",
			watcher.EventInfo{Path: "src/main.go", Event: "Write"},
			watcher.EventInfo{Path: "src/util.go", Event: "Write"},
		)
	}()

	select {
	case evs := <-ch:
		if len(evs) != 2 {
			t.Errorf("expected 2 events, got %d", len(evs))
		}
		if evs[0].Path != "src/main.go" {
			t.Errorf("expected path 'src/main.go', got %q", evs[0].Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mock events")
	}
}

func TestMockRestore(t *testing.T) {
	restore := watcher.Mock()

	// After restore, Mock channels should be nil and Watch restored.
	restore()

	// Dispatch should be a no-op (no panic).
	watcher.Dispatch("nonexistent", watcher.EventInfo{})
}
