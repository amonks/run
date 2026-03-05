// Package printer provides a non-interactive UI for displaying interleaved
// multiplexed streams. The UI prints interleaved output from all of the
// streams to its Stdout. The output is suitable for piping to a file.
package printer

import (
	"context"
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"monks.co/run/internal/color"
	"monks.co/run/internal/mutex"
	"monks.co/run/runner"
)

// New produces a non-interactive UI for displaying interleaved
// multiplexed streams. The UI prints interleaved output from all of the
// streams to its Stdout. The output is suitable for piping to a file.
//
// The gutterWidth parameter controls the width of the task-ID gutter
// (typically the length of the longest task ID). The stdout parameter
// receives the formatted output.
//
// The Printer is safe to access concurrently from multiple goroutines.
func New(gutterWidth int, stdout io.Writer) *Printer {
	return &Printer{
		mu:        mutex.New("printer"),
		stdout:    stdout,
		keyLength: gutterWidth,
	}
}

// Printer is a non-interactive UI that writes interleaved task output to
// a single stream, prefixed with color-coded task IDs.
type Printer struct {
	mu        *mutex.Mutex
	stdout    io.Writer
	keyLength int
	lastKey   string
}

// *Printer implements MultiWriter
var _ runner.MultiWriter = &Printer{}

func (p *Printer) Writer(id string) io.Writer {
	return printerWriter{p, id}
}

var _ io.Writer = printerWriter{}

type printerWriter struct {
	printer *Printer
	id      string
}

func (w printerWriter) Write(bs []byte) (int, error) {
	w.printer.write(w.id, string(bs))
	return len(bs), nil
}

// Start implements [runner.UI]. It signals readiness immediately and
// blocks until the context is canceled.
func (p *Printer) Start(ctx context.Context, ready chan<- struct{}, _ io.Reader, _ io.Writer) error {
	if ready != nil {
		ready <- struct{}{}
	}

	<-ctx.Done()

	return nil
}

func (p *Printer) write(key, message string) {
	defer p.mu.Lock("Write").Unlock()

	if p.stdout == nil {
		panic("nil stdout in printer")
	}

	lines := strings.SplitSeq(message, "\n")
	for l := range lines {
		if l == "" {
			continue
		}
		k := ""
		space := ""
		if key != p.lastKey {
			if p.lastKey != "" {
				space = "\n"
			}
			k, p.lastKey = key, key
		}
		keyStyle := keyStyle
		keyStyle = keyStyle.Foreground(color.Hash(key))
		if p.stdout == nil {
			panic("nil stdout")
		}
		fmt.Fprintln(p.stdout, space+lipgloss.JoinHorizontal(
			lipgloss.Top,
			keyStyle.Width(p.keyLength).Render(k),
			l,
		))
	}
}

var (
	keyStyle = lipgloss.NewStyle().
		Height(1).
		Align(lipgloss.Right).
		Margin(0, 2)
)
