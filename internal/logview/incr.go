package logview

import "sync"

type incr struct {
	n  int
	mu sync.Mutex
}

func (i *incr) incr() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.n += 1
	return i.n
}
