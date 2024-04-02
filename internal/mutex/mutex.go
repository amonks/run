package mutex

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

func New(name string) *Mutex {
	return &Mutex{name: name}
}

// Mutex wraps sync.Mutex, providing these additional features:
//   - You can `defer Lock(...).Unlock()` in a single line
//   - If `debug` is true, Mutex lock/unlock info will be logged to
//     Mutex.log.
//   - You can log additional info to Mutex.log with [Mutex.printf].
type Mutex struct {
	name     string
	holder   string
	mu       sync.Mutex
	internal sync.Mutex
}

var debug = false
var logfile *os.File

func init() {
	if !debug {
		return
	}
	f, err := os.Create("mutex.log")
	if err != nil {
		panic(err)
	}
	logfile = f
}

func (mu *Mutex) Lock(name string) *Mutex {
	mu.printf("%s seeks lock", name)
	mu.mu.Lock()
	mu.printf("%s got lock", name)

	return mu
}

func (mu *Mutex) Unlock() {
	mu.setHolder("")
	mu.printf("unlocks")
	mu.mu.Unlock()
}

func (mu *Mutex) setHolder(name string) {
	mu.internal.Lock()
	defer mu.internal.Unlock()
	mu.holder = name
}

func (mu *Mutex) printf(s string, args ...interface{}) {
	if debug {
		mu.internal.Lock()
		defer mu.internal.Unlock()

		s = strings.TrimSpace(s)
		holder := mu.holder
		var suffix string
		if holder != "" {
			suffix = fmt.Sprintf(" <held by %s>", holder)
		}
		d := time.Now().Format(time.StampNano)
		prefix := fmt.Sprintf("%s [%s] ", d, mu.name)
		fmt.Fprintf(logfile, prefix+s+suffix+"\n", args...)
	}
}
