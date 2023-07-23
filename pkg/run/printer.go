package run

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/charmbracelet/lipgloss"
)

func newPrinter(run *Run) UI {
	return &printer{mu: newMutex("printer"), run: run}
}

type printer struct {
	mu        *mutex
	run       *Run
	stdout    io.Writer
	keyLength int
	lastKey   string
}

// *printer implements MultiWriter
var _ MultiWriter = &printer{}

func (p *printer) Writer(id string) io.Writer {
	return printerWriter{p, id}
}

var _ io.Writer = printerWriter{}

type printerWriter struct {
	printer *printer
	id      string
}

func (w printerWriter) Write(bs []byte) (int, error) {
	w.printer.Write(w.id, string(bs))
	return len(bs), nil
}

func (p *printer) Start(ctx context.Context, ready chan<- struct{}, _ io.Reader, stdout io.Writer) error {
	p.mu.Lock("Write")
	p.stdout = stdout
	p.keyLength = 0
	for _, id := range p.run.IDs() {
		if len(id) > p.keyLength {
			p.keyLength = len(id)
		}
	}
	p.mu.Unlock()

	ready <- struct{}{}

	<-ctx.Done()

	return nil
}

func (p *printer) Write(key, message string) {
	defer p.mu.Lock("Write").Unlock()

	if p.stdout == nil {
		panic("nil stdout in printer")
	}

	lines := strings.Split(message, "\n")
	for _, l := range lines {
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
		keyStyle = keyStyle.Copy().
			Foreground(lipgloss.Color(color.Hash(key)))
		if p.stdout == nil {
			panic("nil stdout")
		}
		fmt.Fprintln(p.stdout, space+lipgloss.JoinHorizontal(
			lipgloss.Top,
			keyStyle.Width(p.keyLength).Render(k),
			valStyle.Render(l),
		))
	}
}

var (
	keyStyle = lipgloss.NewStyle().
			Height(1).
			Align(lipgloss.Right).
			Margin(0, 2).
			Padding(0).
			BorderRight(true)

	valStyle = lipgloss.NewStyle().
			Margin(0).
			Padding(0)
)
