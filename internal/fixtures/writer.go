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

	writer.bufMu.Lock("String-2")
	defer writer.bufMu.Unlock()
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
	w.bufs[id] = newWriter(id, w.combined, w.mu)
	return w.bufs[id]
}

type writer struct {
	tee   io.Writer
	teeMu *mutex.Mutex

	id    string
	buf   *bytes.Buffer
	bufMu *mutex.Mutex
}

func newWriter(id string, tee io.Writer, teeMu *mutex.Mutex) *writer {
	var buf bytes.Buffer
	return &writer{
		tee:   tee,
		teeMu: teeMu,
		id:    id,
		buf:   &buf,
		bufMu: mutex.New("mwwriter"),
	}
}

func (w *writer) Write(bs []byte) (int, error) {
	w.teeMu.Lock("Write")
	w.tee.Write([]byte(fmt.Sprintf("[%s] %s", w.id, string(bs)))) // MARK: read & write
	w.teeMu.Unlock()

	defer w.bufMu.Lock("Write").Unlock()
	return w.buf.Write(bs)
}
