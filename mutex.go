package run

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

func newMutex(name string) *mutex {
	return &mutex{name: name}
}

type mutex struct {
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

func (mu *mutex) printf(s string, args ...interface{}) {
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

func (mu *mutex) Lock(name string) *mutex {
	mu.printf("%s seeks lock", name)
	mu.mu.Lock()
	mu.printf("%s got lock", name)

	return mu
}

func (mu *mutex) Unlock() {
	mu.setHolder("")
	mu.printf("unlocks")
	mu.mu.Unlock()
}

func (mu *mutex) setHolder(name string) {
	mu.internal.Lock()
	defer mu.internal.Unlock()
	mu.holder = name
}
