package outputwriter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/amonks/run/internal/mutex"
)

func New(stdout io.Writer) io.Writer {
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
	w.mu.Lock("Write")
	defer w.mu.Unlock()

	for _, b := range bs {
		if err = w.buf.WriteByte(b); err != nil {
			w.mu.Printf("err")
			return n, err
		}

		n++
		if b == '\n' {
			w.mu.Printf("flush")
			w.buf.Flush()
			w.mu.Printf("flushed")
		}
	}
	w.mu.Printf("done")
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
