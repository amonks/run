package runner

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func newPrinter() UI {
	return &printer{mu: newMutex("printer")}
}

type printer struct {
	mu        *mutex
	stdout    io.Writer
	keyLength int
	lastKey   string
	waiters   []chan<- error
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

func (p *printer) Start(_ io.Reader, stdout io.Writer, ids []string) error {
	defer p.mu.Lock("Start").Unlock()

	p.stdout = stdout
	p.keyLength = 0
	for _, id := range ids {
		if len(id) > p.keyLength {
			p.keyLength = len(id)
		}
	}
	return nil
}

func (p *printer) Wait() <-chan error {
	defer p.mu.Lock("Wait").Unlock()

	c := make(chan error)
	p.waiters = append(p.waiters, c)
	return c
}

func (p *printer) notify(err error) {
	defer p.mu.Lock("notify").Unlock()

	for _, w := range p.waiters {
		select {
		case w <- err:
		default:
		}
		close(w)
	}
}

func (p *printer) Stop() error {
	p.notify(nil)
	return nil
}

func (p *printer) Write(key, message string) {
	defer p.mu.Lock("Write").Unlock()

	if p.stdout == nil {
		panic("nil stdout in printer")
	}

	lines := strings.Split(strings.TrimSpace(message), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
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
			Foreground(lipgloss.Color(colorHash(key)))
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
