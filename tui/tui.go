package tui

import (
	"context"
	"io"

	"github.com/amonks/run/internal/mutex"
	"github.com/amonks/run/printer"
	"github.com/amonks/run/runner"
	"github.com/amonks/run/tasks"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func Start(ctx context.Context, stdin io.Reader, stdout io.Writer, tasks tasks.Library, entryPointTaskIDs ...string) error {
	tui := &_TUI{mu: mutex.New("tui")}
	tui.runner = runner.New(runner.RunnerModeKeepalive, tasks, ".", tui)

	zone.NewGlobal()

	uiReady := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	tuiErr, runnerErr := make(chan error), make(chan error)

	go func() {
		tui.p = tea.NewProgram(
			&Model{
				tui:    tui,
				onInit: func() { uiReady <- struct{}{} },
			},
			tea.WithAltScreen(),
			tea.WithContext(ctx),
			tea.WithFPS(120),
			tea.WithMouseCellMotion())

		interleavedWriter := tui.Writer(runner.InternalTaskInterleaved)
		tui.interleaved = printer.New(tui.runner.Library().LongestID(), interleavedWriter)

		if _, err := tui.p.Run(); err != nil && err != tea.ErrProgramKilled {
			tuiErr <- err
			return
		}
		tuiErr <- nil
	}()

	select {
	case err := <-tuiErr:
		return err
	case <-uiReady:
	}

	go func() {
		if err := tui.runner.Run(ctx, entryPointTaskIDs...); err != nil {
			runnerErr <- err
			return
		}
		runnerErr <- nil
	}()

	select {
	case <-ctx.Done():
		cancel()
		<-runnerErr
		<-tuiErr
		return nil
	case err := <-tuiErr:
		cancel()
		<-runnerErr
		return err
	case err := <-runnerErr:
		cancel()
		<-tuiErr
		return err
	}
}

type _TUI struct {
	mu          *mutex.Mutex
	runner      *runner.Runner
	p           *tea.Program
	interleaved *printer.Printer
}

var _ runner.MultiWriter = &_TUI{}

func (tui *_TUI) Writer(id string) io.Writer {
	tui.mu.Lock("writer")
	defer tui.mu.Unlock()

	if tui.p == nil {
		panic("getting writer from unstarted tui")
	}

	return &tuiWriter{
		mu:                mutex.New("tuiwriter:" + id),
		id:                id,
		interleavedWriter: tui.interleaved,
		send:              tui.p.Send,
	}
}

type tuiWriter struct {
	mu                *mutex.Mutex
	id                string
	interleavedWriter runner.MultiWriter
	send              func(tea.Msg)
}

var _ io.Writer = &tuiWriter{}

func (w *tuiWriter) Write(bs []byte) (int, error) {
	w.mu.Lock("Write")
	defer w.mu.Unlock()

	if w.send == nil {
		panic("nil send")
	}
	if w.id != runner.InternalTaskInterleaved {
		if w.interleavedWriter == nil {
			panic("nil interleaved writer")
		}
		w.interleavedWriter.Writer(w.id).Write(bs)
	}
	go w.send(msgWrite{key: w.id, content: string(bs)})
	return len(bs), nil
}

//go:generate go run github.com/amonks/run/cmd/messagestringer -file $GOFILE -prefix msg
type (
	msgInitialized struct{}
	msgWrite       struct {
		key     string
		content string
	}
)
