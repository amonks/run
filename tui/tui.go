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
	"github.com/amonks/run/task"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// Start creates an interactive terminal UI and a [runner.Run], wires them
// together, and blocks until the user quits or the context is canceled.
//
// The run uses [runner.RunTypeLong], so it keeps running and restarts
// failed tasks until the user exits the TUI.
func Start(ctx context.Context, stdin io.Reader, stdout io.Writer, dir string, allTasks task.Library, taskID string) error {
	zone.NewGlobal()

	t := &tui{mu: mutex.New("tui")}

	r, err := runner.New(runner.RunTypeLong, dir, allTasks, taskID, t)
	if err != nil {
		return err
	}
	t.run = r

	ids := append([]string{runner.InternalTaskInterleaved}, r.IDs()...)

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	runDone := make(chan error, 1)

	program := tea.NewProgram(
		&tuiModel{
			tui: t,
			ids: ids,
			onInit: func() {
				go func() {
					runDone <- r.Start(runCtx)
				}()
			},
		},
		tea.WithContext(ctx),
		tea.WithFPS(120),
	)
	t.p = program

	// Compute gutter width for the interleaved printer.
	gutterWidth := 0
	for _, id := range ids {
		if len(id) > gutterWidth {
			gutterWidth = len(id)
		}
	}

	interleavedWriter := t.Writer(runner.InternalTaskInterleaved)
	t.interleaved = printer.New(gutterWidth, interleavedWriter)

	// Run the BubbleTea program (blocking). The runner starts from the
	// onInit callback once the program's event loop is active.
	_, programErr := program.Run()

	// Program exited (user quit). Cancel the runner.
	runCancel()

	// Wait for the runner to finish.
	runErr := <-runDone

	if programErr != nil && programErr != tea.ErrProgramKilled {
		return programErr
	}
	return runErr
}

type tui struct {
	mu *mutex.Mutex

	run *runner.Run

	// nil until started
	p *tea.Program

	interleaved runner.MultiWriter
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
