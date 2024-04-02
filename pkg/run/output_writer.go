package run

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/amonks/run/internal/mutex"
)

func newOutputWriter(stdout io.Writer) io.Writer {
	jsonW := &jsonWriter{w: stdout}
	bufW := &lineBufferedWriter{buf: bufio.NewWriter(jsonW)}
	bufW.mu = mutex.New("linebuffered")
	return bufW
}

type lineBufferedWriter struct {
	buf *bufio.Writer
	mu  *mutex.Mutex
}

func (w *lineBufferedWriter) Write(bs []byte) (n int, err error) {
	defer w.mu.Lock("Writer").Unlock()
	for _, b := range bs {
		if err = w.buf.WriteByte(b); err != nil {
			return n, err
		}

		n++
		if b == '\n' {
			w.buf.Flush()
		}
	}
	return n, err
}

type jsonWriter struct {
	w io.Writer
}

func (w *jsonWriter) Write(bs []byte) (int, error) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, bs, "", "  "); err == nil {
		w.w.Write(pretty.Bytes())
	} else {
		w.w.Write(bs)
	}
	return len(bs), nil
}
