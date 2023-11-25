package logview

import tea "github.com/charmbracelet/bubbletea"

type writeMsg struct {
	id      int
	content string
}

func (m *Model) WriteMsg(s string) tea.Msg {
	return writeMsg{
		id:      m.id,
		content: s,
	}
}

type setDimensionsMsg struct {
	id     int
	width  int
	height int
}

func (m *Model) SetDimensionsMsg(width, height int) tea.Msg {
	return setDimensionsMsg{
		id:     m.id,
		width:  width,
		height: height,
	}
}

type FocusArea int

const (
	focusNone FocusArea = iota
	FocusSearchBar
	FocusLogPane
)

type setFocusMsg struct {
	id    int
	focus FocusArea
}

func (m *Model) SetFocusMsg(f FocusArea) tea.Msg {
	return setFocusMsg{m.id, f}
}

type setQueryMsg struct {
	id    int
	query string
}

func (m *Model) SetQueryMsg(s string) tea.Msg {
	return setQueryMsg{m.id, s}
}

type moveResultIndexMsg struct {
	id    int
	index int
}

func (m *Model) MoveResultIndexMsg(by int) tea.Msg {
	return setResultIndexMsg{m.id, m.resultIndex + by}
}

type setResultIndexMsg struct {
	id    int
	index int
}

func (m *Model) SetResultIndexMsg(index int) tea.Msg {
	return setResultIndexMsg{m.id, index}
}

type scrollToMsg struct {
	id   int
	line int
}

func (m *Model) ScrollToMsg(line int) tea.Msg {
	return scrollToMsg{m.id, line}
}

type scrollByMsg struct {
	id    int
	lines int
}

func (m *Model) ScrollByMsg(lines int) tea.Msg {
	return scrollByMsg{m.id, lines}
}

type toggleWrapModeMsg struct {
	id int
}

func (m *Model) ToggleWrapModeMsg() tea.Msg {
	return setWrapModeMsg{m.id, !m.shouldHardwrap}
}

type setWrapModeMsg struct {
	id       int
	hardwrap bool
}

func (m *Model) SetWrapModeMsg(hardwrap bool) tea.Msg {
	return setWrapModeMsg{m.id, hardwrap}
}
