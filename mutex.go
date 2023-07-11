package run

import (
	"fmt"
	"os"
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
		d := time.Now().Format(time.StampNano)
		fmt.Fprintf(logfile, d+" "+s, args...)
	}
}

func (mu *mutex) Lock(name string) *mutex {
	mu.internal.Lock()
	holder := mu.holder
	muName := mu.name
	mu.internal.Unlock()

	var suffix string
	if holder != "" {
		suffix = " from " + holder
	}

	mu.printf("[%s] %s seeks lock%s\n", muName, name, suffix)
	mu.mu.Lock()
	mu.printf("[%s] %s got lock%s\n", muName, name, suffix)

	mu.internal.Lock()
	mu.holder = name
	mu.internal.Unlock()

	return mu
}

func (mu *mutex) Unlock() {
	mu.internal.Lock()
	mu.printf("[%s] %s unlocks\n", mu.name, mu.holder)
	mu.holder = ""
	mu.mu.Unlock()
	mu.internal.Unlock()
}
