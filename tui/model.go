package tui

import (
	"github.com/amonks/run/logview"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	tui *_TUI

	onInit func()

	focus               focusArea
	selectedTaskIDIndex int
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

func (m *Model) Init() tea.Cmd {
	m.tasks = map[string]*logview.Model{}
	for _, id := range m.tui.runner.Library().IDs() {
		lv := logview.New(logview.WithoutStatusbar)
		lv.SetWrapMode(true)
		m.tasks[id] = lv
	}
	for _, id := range []string{"@interleaved", "@watch"} {
		lv := logview.New(logview.WithoutStatusbar)
		lv.SetWrapMode(true)
		m.tasks[id] = lv
	}

	m.help = viewport.New(m.width, m.height)

	m.shortSpinner = spinner.New()
	m.shortSpinner.Spinner = spinner.Jump
	m.longSpinner = spinner.New()
	m.longSpinner.Spinner = spinner.Hamburger

	m.longestIDLength = m.tui.runner.Library().LongestID()

	m.didInit = true

	cmd := func() tea.Msg { return msgInitialized{} }
	return tea.Batch(cmd, m.shortSpinner.Tick, m.longSpinner.Tick)
}

func (m *Model) visibleMenuItems() []string {
	return nil
}

func (m *Model) activeTask() *logview.Model {
	return m.tasks[m.activeTaskID()]
}

func (m *Model) activeTaskID() string {
	return m.tui.runner.Status().AllTasks[m.selectedTaskIDIndex]
}

type focusArea int

const (
	focusMenu focusArea = iota
	focusLogs
	focusHelp
	focusSearch
)
