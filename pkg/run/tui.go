package run

import (
	"context"
	"io"

	"github.com/amonks/run/internal/mutex"
	"github.com/amonks/run/pkg/logview"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func newTUI(run *Run) UI {
	zone.NewGlobal()
	return &tui{mu: mutex.New("tui"), run: run}
}

type tui struct {
	mu *mutex.Mutex

	run *Run

	// nil until started
	p *tea.Program

	interleaved UI
}

// *tui implements MultiWriter
var _ MultiWriter = &tui{}

func (a *tui) Writer(id string) io.Writer {
	defer a.mu.Lock("Writer").Unlock()

	if a.p == nil {
		panic("getting writer from unstarted tui")
	}

	return tuiWriter{
		mu:                mutex.New("writer:" + id),
		id:                id,
		interleavedWriter: a.interleaved,
		send:              a.p.Send,
	}
}

// *writer implements io.Writer
var _ io.Writer = tuiWriter{}

type tuiWriter struct {
	mu                *mutex.Mutex
	id                string
	interleavedWriter MultiWriter
	send              func(tea.Msg)
}

func (w tuiWriter) Write(bs []byte) (int, error) {
	defer w.mu.Lock("Write").Unlock()

	if w.send == nil {
		panic("nil send")
	}
	if w.id != "interleaved" {
		if w.interleavedWriter == nil {
			panic("nil interleaved writer")
		}
		w.interleavedWriter.Writer(w.id).Write(bs)
	}
	w.send(writeMsg{key: w.id, content: string(bs)})
	return len(bs), nil
}

// *tui implements UI
var _ UI = &tui{}

func (a *tui) Start(ctx context.Context, ready chan<- struct{}, stdin io.Reader, stdout io.Writer) error {
	program := tea.NewProgram(
		&tuiModel{
			tui:    a,
			ids:    append([]string{"interleaved"}, a.run.IDs()...),
			onInit: func() { ready <- struct{}{} },
		},
		tea.WithAltScreen(),
		tea.WithContext(ctx),
		tea.WithMouseCellMotion())
	a.p = program

	interleavedWriter := a.Writer("interleaved")
	p := NewPrinter(a.run)
	go p.Start(ctx, nil, nil, interleavedWriter)
	a.interleaved = p

	exit := make(chan error)

	go func() {
		// run the bubbletea Program
		if _, err := program.Run(); err != nil && err != tea.ErrProgramKilled {
			exit <- err
			return
		}

		// When it exits, notify Waiters that the UI is done.
		exit <- nil
	}()

	err := <-exit
	return err
}

type tuiModel struct {
	tui *tui

	onInit func()

	focus               focusArea
	ids                 []string
	selectedTaskIDIndex int
	longestIDLength     int

	width  int
	height int

	lastkey string
	didInit bool
	gotSize bool

	tasks map[string]*logview.Model

	shortSpinner spinner.Model
	longSpinner  spinner.Model

	help viewport.Model
}

type focusArea int

const (
	focusMenu focusArea = iota
	focusLogs
	focusHelp
	focusSearch
)

func (m *tuiModel) activeTask() *logview.Model {
	return m.tasks[m.activeTaskID()]
}

func (m *tuiModel) activeTaskID() string {
	return m.ids[m.selectedTaskIDIndex]
}

func (m *tuiModel) Init() tea.Cmd {
	m.tasks = map[string]*logview.Model{}
	for _, id := range m.ids {
		lv := logview.New(logview.WithoutStatusbar)
		lv.SetWrapMode(true)
		m.tasks[id] = lv
	}

	m.help = viewport.New(m.width, m.height)

	m.shortSpinner = spinner.New()
	m.shortSpinner.Spinner = spinner.Jump
	m.longSpinner = spinner.New()
	m.longSpinner.Spinner = spinner.Hamburger

	for _, id := range m.ids {
		if len(id) > m.longestIDLength {
			m.longestIDLength = len(id)
		}
	}
	m.didInit = true

	cmd := func() tea.Msg { return initializedMsg{} }
	return tea.Batch(cmd, m.shortSpinner.Tick, m.longSpinner.Tick)
}

type (
	initializedMsg struct{}
	writeMsg       struct {
		key     string
		content string
	}
)
