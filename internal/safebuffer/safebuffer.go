package safebuffer

import (
	"bytes"
	"sync"
)

func New() *safeBuffer {
	return &safeBuffer{}
}

type safeBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(bs []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(bs)
}

func (sb *safeBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.String()
}
