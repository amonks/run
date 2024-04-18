package runner

import (
	"io"

	"github.com/amonks/run/internal/mutex"
)

type MultiWriter interface {
	Writer(id string) io.Writer
}

func wrapMultiWriter(mw MultiWriter, fn func(io.Writer) io.Writer) MultiWriter {
	return &wrappedMultiWriter{
		mu:      mutex.New("wrappedmultiwriter"),
		base:    mw,
		wrapfn:  fn,
		writers: map[string]io.Writer{},
	}
}

type wrappedMultiWriter struct {
	base    MultiWriter
	wrapfn  func(io.Writer) io.Writer
	writers map[string]io.Writer
	mu      *mutex.Mutex
}

var _ MultiWriter = &wrappedMultiWriter{}

func (wmw *wrappedMultiWriter) Writer(id string) io.Writer {
	wmw.mu.Lock("Writer")
	defer wmw.mu.Unlock()

	if w, has := wmw.writers[id]; has {
		return w
	}

	wmw.mu.Printf("call wrapfn")
	wmw.writers[id] = wmw.wrapfn(wmw.base.Writer(id))
	wmw.mu.Printf("called wrapfn")

	return wmw.writers[id]
}
