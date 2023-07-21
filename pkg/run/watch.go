package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/glob"
	"github.com/rjeczalik/notify"
)

type eventInfo struct {
	path  string
	event string
}

type watcher struct{}

func (w *watcher) watch(inputPath string) (<-chan []eventInfo, func(), error) {
	if strings.HasPrefix(inputPath, "/") {
		return nil, nil, fmt.Errorf("watch '%s' is invalid because it is not a relative path", inputPath)
	} else if strings.Contains(inputPath, "..") {
		return nil, nil, fmt.Errorf("watch '%s' is invalid because it contains the substring '..'", inputPath)
	}

	var stopped bool

	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}

	watchPath, globToMatch := w.split(inputPath)

	// Start listening for events
	c := make(chan notify.EventInfo, 1)
	out := make(chan eventInfo)

	go func() {
		for ev := range c {
			p := strings.TrimPrefix(ev.Path(), cwd+"/")
			if globToMatch == nil || globToMatch.Match(p) {
				out <- eventInfo{
					path:  p,
					event: strings.TrimPrefix(ev.Event().String(), "notify."),
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
	}

	return w.debounce(500*time.Millisecond, out), stop, nil
}

type debounced[T any] struct {
	mu      sync.Mutex
	coll    []T
	waiting bool
}

func (*watcher) debounce(dur time.Duration, c <-chan eventInfo) <-chan []eventInfo {
	debounced := &debounced[eventInfo]{}

	debouncedC := make(chan []eventInfo)

	go func() {
		for ev := range c {
			debounced.mu.Lock()
			debounced.coll = append(debounced.coll, ev)
			if debounced.waiting {
				debounced.mu.Unlock()
				continue
			}
			debounced.waiting = true
			debounced.mu.Unlock()

			go func() {
				time.Sleep(dur)

				debounced.mu.Lock()
				defer debounced.mu.Unlock()

				select {
				case debouncedC <- debounced.coll:
				default:
				}
				debounced.coll = nil
				debounced.waiting = false
			}()
		}
	}()

	return debouncedC
}

// Split breaks a given input path (which may contain a glob) into two parts: a
// watcher part and a glob part.
//
// For example, given the input "src/website/**/*.js",
//  - we will set up a recursive watch at src/website
//  - we will match events from that watch against the glob "src/website/**/*.js"
// so the values returned from split will be ("src/website", Glob["src/website/**/*.js"]).
func (*watcher) split(input string) (string, glob.Glob) {
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
