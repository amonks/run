package mutex

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

func New(name string) *Mutex {
	mu := &Mutex{name: name}
	mu.Printf("--- begin ---")
	return mu
}

// Mutex wraps sync.Mutex, providing these additional features:
//   - You can `defer Lock(...).Unlock()` in a single line
//   - If `debug` is true, Mutex lock/unlock info will be logged to
//     Mutex.log.
//   - You can log additional info to Mutex.log with [Mutex.printf].
type Mutex struct {
	name string
	mu   sync.Mutex
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
	mu.Printf("%s seeks lock", name)
	mu.mu.Lock()
	mu.Printf("%s receives lock", name)

	return mu
}

func (mu *Mutex) Unlock() {
	mu.Printf("releases lock")
	mu.mu.Unlock()
}

func (mu *Mutex) Printf(s string, args ...interface{}) {
	if debug {
		s = strings.TrimSpace(s)
		var suffix string
		d := time.Now().Format(time.StampNano)
		prefix := fmt.Sprintf("%s [%s] ", d, mu.name)
		fmt.Fprintf(logfile, prefix+s+suffix+"\n", args...)
	}
}
