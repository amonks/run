// Scroll is a tailing pager.
//
//   - Unlike tail, it has interactive features like
//     paging and incremental regex search.
//
//   - Unlike less, it continuously tries to keep reading
//     more bytes from the input file.
//
//   - Unlike less and tail, it loads the whole file into
//     memory before starting, which makes it perform worse
//     on files which are already large.
//
// On my machine, I start to notice perceptible latency:
//   - Changing search query in a file larger than a few dozen MB
//   - Opening a file larger than a few hundred MB
//
// I never notice latency in scrolling or tailing, even in
// files up to 15 GB. I didn't test any files larger than that
// because they take too long to open.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/amonks/run/pkg/logview"
	tea "github.com/charmbracelet/bubbletea/v2"
)

func main() {
	flag.Parse()

	program := tea.NewProgram(newScroll(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	filename := flag.Arg(0)

	sinkErr := make(chan error)
	sink := Sink(func(s string) { program.Send(writeMsg(s)) })
	go func() {
		switch filename {
		case "-", "":
			sinkErr <- sink.tailStdin()
		default:
			sinkErr <- sink.tailFile(filename)
		}
	}()

	programErr := make(chan error)
	go func() {
		_, err := program.Run()
		programErr <- err
	}()

	select {
	case err := <-sinkErr:
		program.Send(tea.Quit())
		<-programErr
		fmt.Println(err)
		os.Exit(1)
	case err := <-programErr:
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}

type (
	writeMsg string
)

type scroll struct {
	logview *logview.Model
}

func newScroll() *scroll {
	return &scroll{logview.New()}
}

var _ tea.Model = &scroll{}

func (t *scroll) Init() tea.Cmd { return t.logview.Init() }

func (t *scroll) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.logview.SetDimensions(msg.Width, msg.Height)
		return t, nil
	case writeMsg:
		t.logview.Write(string(msg))
		return t, nil
	}
	model, cmd := t.logview.Update(msg)
	t.logview = model.(*logview.Model)
	return t, cmd
}

func (t *scroll) View() string { return t.logview.View() }
