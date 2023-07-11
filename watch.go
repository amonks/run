package runner

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
)

type eventInfo struct {
	path  string
	event string
}

func watch(watchPath string) (<-chan []eventInfo, func(), error) {
	var stopped bool

	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}

	// Start listening for events
	c := make(chan notify.EventInfo)
	out := make(chan eventInfo)

	go func() {
		for ev := range c {
			out <- eventInfo{
				path:  strings.TrimPrefix(ev.Path(), cwd+"/"),
				event: strings.TrimPrefix(ev.Event().String(), "notify."),
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

	return debounce(500*time.Millisecond, out), stop, nil
}

type debounced[T any] struct {
	mu      sync.Mutex
	coll    []T
	waiting bool
}

func debounce[T any](dur time.Duration, c <-chan T) <-chan []T {
	debounced := &debounced[T]{}

	debouncedC := make(chan []T)

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
