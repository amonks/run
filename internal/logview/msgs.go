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

type searchMsg struct {
	id    int
	query string
}

func (m *Model) SearchMsg(s string) tea.Msg {
	return searchMsg{m.id, s}
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

type setWrapModeMsg struct {
	id       int
	hardwrap bool
}

func (m *Model) SetWrapModeMsg(hardwrap bool) tea.Msg {
	return setWrapModeMsg{m.id, hardwrap}
}
