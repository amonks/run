package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/internal/mutex"
	"github.com/amonks/run/runner"
	"github.com/charmbracelet/lipgloss"
)

type Printer struct {
	mu          *mutex.Mutex
	stdout      io.Writer
	gutterWidth int
	lastKey     string
}

func New(gutterWidth int, stdout io.Writer) *Printer {
	return &Printer{
		mu:          mutex.New("printer"),
		gutterWidth: gutterWidth,
		stdout:      stdout,
	}
}

func (p *Printer) Write(key, message string) {
	p.mu.Lock("Write:" + key)
	defer p.mu.Unlock()

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
			Foreground(color.Hash(key))
		if p.stdout == nil {
			panic("nil stdout")
		}
		fmt.Fprintln(p.stdout, space+lipgloss.JoinHorizontal(
			lipgloss.Top,
			keyStyle.Width(p.gutterWidth).Render(k),
			l,
		))
	}
}

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
	w.printer.Write(w.id, string(bs))
	return len(bs), nil
}

var (
	keyStyle = lipgloss.NewStyle().
		Height(1).
		Align(lipgloss.Right).
		Margin(0, 2)
)
