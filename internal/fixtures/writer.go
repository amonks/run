package fixtures

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sync"
)

// ansiRegexp matches ANSI escape codes. Duplicated from pkg/run/ansi.go
// since it is unexported.
var ansiRegexp = regexp.MustCompile(
	"[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))",
)

// Writer is a test MultiWriter that records per-stream and combined output.
// It strips ANSI escape codes so that assertions don't need to account for
// styling.
type Writer struct {
	mu       sync.Mutex
	streams  map[string]*bytes.Buffer
	combined bytes.Buffer
}

// NewWriter creates a new recording Writer.
func NewWriter() *Writer {
	return &Writer{
		streams: make(map[string]*bytes.Buffer),
	}
}

// Writer returns an io.Writer for the given stream ID. Each write is
// recorded both in the per-stream buffer and in the combined buffer
// (prefixed with "[id] ").
func (w *Writer) Writer(id string) io.Writer {
	w.mu.Lock()
	if _, ok := w.streams[id]; !ok {
		w.streams[id] = &bytes.Buffer{}
	}
	w.mu.Unlock()
	return &streamWriter{parent: w, id: id}
}

// String returns the raw output for a single stream, with ANSI codes
// stripped.
func (w *Writer) String(id string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	buf, ok := w.streams[id]
	if !ok {
		return ""
	}
	return buf.String()
}

// CombinedString returns all output interleaved with [id] prefixes, with
// ANSI codes stripped.
func (w *Writer) CombinedString() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.combined.String()
}

type streamWriter struct {
	parent *Writer
	id     string
}

func (sw *streamWriter) Write(p []byte) (int, error) {
	clean := ansiRegexp.ReplaceAll(p, nil)

	sw.parent.mu.Lock()
	defer sw.parent.mu.Unlock()

	buf, ok := sw.parent.streams[sw.id]
	if !ok {
		buf = &bytes.Buffer{}
		sw.parent.streams[sw.id] = buf
	}
	buf.Write(clean)
	fmt.Fprintf(&sw.parent.combined, "[%s] %s", sw.id, clean)
	return len(p), nil
}
