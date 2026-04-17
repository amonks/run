// Package watcher provides file system watching with debouncing and glob
// matching. It also provides mock support for testing.
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/glob"
	"github.com/rjeczalik/notify"
)

// EventInfo describes a single file system event.
type EventInfo struct {
	Path  string
	Event string
}

// Watch observes the file system at inputPath and returns a channel of
// debounced events, a stop function, and any error. The inputPath may contain
// glob patterns (e.g., "src/website/**/*.js").
//
// Watch is a package-level function variable to allow replacement for testing
// via Mock.
var Watch = func(inputPath string) (<-chan []EventInfo, func(), error) {
	var stopped bool

	cwdRaw, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	// kqueue (FreeBSD, NetBSD, OpenBSD) and FSEvents report event paths
	// with symlinks resolved. If the cwd is reached through a symlink
	// (e.g. FreeBSD's /home -> /usr/home), the raw cwd will not be a
	// prefix of the event path, so also keep the resolved form.
	cwdResolved, rerr := filepath.EvalSymlinks(cwdRaw)
	if rerr != nil {
		cwdResolved = ""
	}

	watchPath, globToMatch := Split(inputPath)

	// Start listening for events.
	c := make(chan notify.EventInfo, 1)
	out := make(chan EventInfo)

	go func() {
		for ev := range c {
			p := StripCwd(ev.Path(), cwdRaw, cwdResolved)
			if globToMatch == nil || globToMatch.Match(p) {
				out <- EventInfo{
					Path:  p,
					Event: strings.TrimPrefix(ev.Event().String(), "notify."),
				}
			}
		}
		close(out)
	}()

	stop := func() {
		if stopped {
			return
		}
		stopped = true
		notify.Stop(c)
		close(c)
	}

	// Start the watcher.
	if err := notify.Watch(watchPath, c, notify.All); err != nil {
		stop()
		return nil, nil, err
	}

	return Debounce(500*time.Millisecond, out), stop, nil
}

// StripCwd returns eventPath made relative to the current working directory.
// It tries the raw Getwd value first, then the symlink-resolved form, to
// handle OSes where the watcher reports realpaths (kqueue, FSEvents) and
// the cwd is reached through a symlink. Unmatched paths are returned
// unchanged.
func StripCwd(eventPath, cwdRaw, cwdResolved string) string {
	if s := strings.TrimPrefix(eventPath, cwdRaw+"/"); s != eventPath {
		return s
	}
	if cwdResolved != "" && cwdResolved != cwdRaw {
		if s := strings.TrimPrefix(eventPath, cwdResolved+"/"); s != eventPath {
			return s
		}
	}
	return eventPath
}

type debounced[T any] struct {
	mu      sync.Mutex
	coll    []T
	waiting bool
}

// Debounce collects events from c and emits them as batches after dur of
// inactivity.
func Debounce(dur time.Duration, c <-chan EventInfo) <-chan []EventInfo {
	d := &debounced[EventInfo]{}
	debouncedC := make(chan []EventInfo)

	go func() {
		for ev := range c {
			d.mu.Lock()
			d.coll = append(d.coll, ev)
			if d.waiting {
				d.mu.Unlock()
				continue
			}
			d.waiting = true
			d.mu.Unlock()

			go func() {
				time.Sleep(dur)

				d.mu.Lock()
				defer d.mu.Unlock()

				select {
				case debouncedC <- d.coll:
				default:
				}
				d.coll = nil
				d.waiting = false
			}()
		}
	}()

	return debouncedC
}

// Split breaks a given input path (which may contain a glob) into two parts:
// a watch path suitable for the file system watcher and an optional glob for
// filtering events.
//
// For example, given the input "src/website/**/*.js",
//   - we will set up a recursive watch at src/website
//   - we will match events against the glob "src/website/**/*.js"
//
// so the values returned are ("src/website/...", Glob["src/website/**/*.js"]).
func Split(input string) (string, glob.Glob) {
	input = filepath.Clean(input)
	segments := strings.Split(input, "/")
	for i, seg := range segments {
		if strings.Contains(seg, "*") {
			w := strings.Join(segments[:i], "/")
			return filepath.Join(w, "..."), glob.MustCompile(input)
		}
	}
	return input, nil
}

// --- Mock support ---

var (
	mockMu       sync.Mutex
	mockChannels map[string]chan []EventInfo
)

// Mock replaces Watch with an implementation that captures calls and allows
// synthetic events via Dispatch. It returns a restore function that must be
// called to reinstate the real Watch.
func Mock() func() {
	mockMu.Lock()
	defer mockMu.Unlock()

	original := Watch
	mockChannels = make(map[string]chan []EventInfo)

	Watch = func(inputPath string) (<-chan []EventInfo, func(), error) {
		mockMu.Lock()
		defer mockMu.Unlock()

		ch := make(chan []EventInfo, 16)
		mockChannels[inputPath] = ch
		stop := func() {
			mockMu.Lock()
			defer mockMu.Unlock()
			delete(mockChannels, inputPath)
		}
		return ch, stop, nil
	}

	return func() {
		mockMu.Lock()
		defer mockMu.Unlock()
		Watch = original
		mockChannels = nil
	}
}

// Dispatch sends synthetic events to the mock watcher for the given path.
// The path must match the inputPath previously passed to Watch.
func Dispatch(path string, evs ...EventInfo) {
	mockMu.Lock()
	ch, ok := mockChannels[path]
	mockMu.Unlock()
	if ok {
		ch <- evs
	}
}
