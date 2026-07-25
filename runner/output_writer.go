package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"monks.co/run/internal/mutex"
)

// newOutputWriter wraps stdout with line buffering. When pretty is true, whole
// JSON lines are re-indented for human reading (the interactive default). When
// pretty is false — non-interactive output piped to a file or shipped to syslog
// by a supervisor — lines pass through verbatim so one log event stays one line.
func newOutputWriter(stdout io.Writer, pretty bool) io.Writer {
	dst := stdout
	if pretty {
		dst = &jsonWriter{w: stdout}
	}
	bufW := &lineBufferedWriter{buf: bufio.NewWriter(dst)}
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
