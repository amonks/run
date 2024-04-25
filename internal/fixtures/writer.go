package fixtures

import (
	"bytes"
	"fmt"
	"io"

	"github.com/amonks/run/internal/mutex"
)

type MultiWriter struct {
	combined *bytes.Buffer
	bufs     map[string]*writer
	mu       *mutex.Mutex
}

func NewWriter() *MultiWriter {
	var buf bytes.Buffer
	return &MultiWriter{
		combined: &buf,
		mu:       mutex.New("testwriter"),
	}
}

func (w *MultiWriter) Write(bs []byte) (int, error) {
	w.mu.Lock("Write")
	defer w.mu.Unlock()

	return w.combined.Write(bs)
}

func (w *MultiWriter) String(id string) string {
	w.mu.Lock("String")
	defer w.mu.Unlock()

	writer, hasWriter := w.bufs[id]
	if !hasWriter {
		return ""
	}

	writer.mu.Lock("String-2")
	defer writer.mu.Unlock()
	return writer.buf.String()
}

func (w *MultiWriter) CombinedString() string {
	w.mu.Lock("CombinedString")
	defer w.mu.Unlock()

	return w.combined.String()
}

func (w *MultiWriter) Writer(id string) io.Writer {
	w.mu.Lock("Writer:" + id)
	defer w.mu.Unlock()

	if w.bufs == nil {
		w.bufs = make(map[string]*writer)
	}
	if w, exists := w.bufs[id]; exists {
		return w
	}
	w.bufs[id] = newWriter(id, w.combined)
	return w.bufs[id]
}

type writer struct {
	tee io.Writer
	id  string
	mu  *mutex.Mutex
	buf *bytes.Buffer
}

func newWriter(id string, tee io.Writer) *writer {
	var buf bytes.Buffer
	return &writer{tee: tee, id: id, buf: &buf}
}

func (w *writer) Write(bs []byte) (int, error) {
	w.tee.Write([]byte(fmt.Sprintf("[%s] %s", w.id, string(bs))))
	return w.buf.Write(bs)
}
