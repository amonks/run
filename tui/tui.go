// Package tui provides an interactive terminal UI for displaying multiplexed
// streams. The UI shows a list of the streams, and allows keyboard and mouse
// navigation for selecting a particular stream to inspect.
package tui

import (
	"context"
	"io"
	"time"

	"github.com/amonks/run/internal/mutex"
	"github.com/amonks/run/logview"
	"github.com/amonks/run/printer"
	"github.com/amonks/run/runner"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// New produces an interactive terminal UI for displaying multiplexed
// streams. The UI shows a list of the streams, and allows keyboard and mouse
// navigation for selecting a particular stream to inspect.
//
// The UI can be passed into [runner.Run.Start] to display a run's execution.
//
// The UI is safe to access concurrently from multiple goroutines.
func New(run *runner.Run) runner.UI {
	zone.NewGlobal()
	return &tui{mu: mutex.New("tui"), run: run}
}

type tui struct {
	mu *mutex.Mutex

	run *runner.Run

	// nil until started
	p *tea.Program

	interleaved runner.UI
}

// *tui implements MultiWriter
var _ runner.MultiWriter = &tui{}

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
	interleavedWriter runner.MultiWriter
	send              func(tea.Msg)
}

func (w tuiWriter) Write(bs []byte) (int, error) {
	defer w.mu.Lock("Write").Unlock()

	if w.send == nil {
		panic("nil send")
	}
	if w.id != runner.InternalTaskInterleaved {
		if w.interleavedWriter == nil {
			panic("nil interleaved writer")
		}
		w.interleavedWriter.Writer(w.id).Write(bs)
	}
	w.send(writeMsg{key: w.id, content: string(bs)})
	return len(bs), nil
}

// *tui implements UI
var _ runner.UI = &tui{}

func (a *tui) Start(ctx context.Context, ready chan<- struct{}, stdin io.Reader, stdout io.Writer) error {
	program := tea.NewProgram(
		&tuiModel{
			tui:    a,
			ids:    append([]string{runner.InternalTaskInterleaved}, a.run.IDs()...),
			onInit: func() { ready <- struct{}{} },
		},
		tea.WithContext(ctx),
		tea.WithFPS(120))
	a.p = program

	interleavedWriter := a.Writer(runner.InternalTaskInterleaved)
	p := printer.New(a.run)
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
	menuScrollOffset    int
	longestIDLength     int

	width  int
	height int

	quitKey string
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

	m.help = viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(m.height))

	m.shortSpinner = spinner.New()
	m.shortSpinner.Spinner = spinner.Jump
	m.longSpinner = spinner.New()
	m.longSpinner.Spinner = spinner.Spinner{
		Frames: []string{"⣤", "⣠", "⣄", "⡤", "⣤", "⡤", "⢤", "⣠"},
		FPS:    time.Second / 4,
	}

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
