package runner

import (
	"sync"
	"time"

	"github.com/rjeczalik/notify"
)

func watch(path string) (<-chan []notify.EventInfo, func()) {
	var stopped bool

	// Start listening for events
	c := make(chan notify.EventInfo)

	stop := func() {
		if stopped {
			return
		}
		stopped = true
		notify.Stop(c)
		close(c)
	}

	// Start the watcher.
	if err := notify.Watch(path, c, notify.All); err != nil {
		stop()
	}

	return debounce(500*time.Millisecond, c), stop
}

type debounced struct {
	mu      sync.Mutex
	coll    []notify.EventInfo
	waiting bool
}

func debounce(dur time.Duration, c <-chan notify.EventInfo) <-chan []notify.EventInfo {
	debounced := &debounced{}

	debouncedC := make(chan []notify.EventInfo)

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
