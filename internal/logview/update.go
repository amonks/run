package logview

import (
	"bufio"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.shouldHandleKeyboard {
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			msg = m.translateKey(kmsg)
		}
	}

	if m.shouldHandleMouse {
		if mmsg, ok := msg.(tea.MouseMsg); ok {
			msg = m.translateMouse(tea.MouseEvent(mmsg))
		}
	}

	switch msg := msg.(type) {
	case writeMsg:
		m.handleWrite(msg)
	case searchMsg:
		m.handleSearch(msg)
	case tea.WindowSizeMsg:
		m.windowWidth, m.windowHeight = msg.Width, msg.Height
	case scrollByMsg:
		// if tailing, set scroll position to bottom
		if m.scrollPosition < 0 {
			m.scrollPosition = len(m.lines) - m.logHeight() - 1
			return m, nil
		}

		// update scroll position
		m.scrollPosition = max(0, m.scrollPosition+msg.lines)

		// become tailing if scrolled all the way down
		if m.scrollPosition > len(m.lines)-m.logHeight() {
			m.scrollPosition = -1
		}
	case scrollToMsg:
		if msg.line < 0 {
			m.scrollPosition = -1
		} else {
			m.scrollPosition = msg.line
		}
	case setWrapModeMsg:
		m.shouldHardwrap = msg.hardwrap
	case tea.QuitMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m *Model) translateKey(msg tea.KeyMsg) tea.Msg {
	switch msg.String() {
	case "down", "j":
		return m.ScrollByMsg(1)
	case "up", "k":
		return m.ScrollByMsg(-1)
	case "w":
		return m.SetWrapModeMsg(!m.shouldHardwrap)
	case "G":
		return m.ScrollToMsg(-1)
	case "ctrl+c", "q":
		return tea.Quit()
	default:
		return msg
	}
}

func (m *Model) translateMouse(msg tea.MouseEvent) tea.Msg {
	switch msg.Type {
	case tea.MouseWheelDown:
		return m.ScrollByMsg(1)
	case tea.MouseWheelUp:
		return m.ScrollByMsg(-1)
	default:
		return msg
	}
}

func (m *Model) handleWrite(msg writeMsg) {
	scanner := bufio.NewScanner(strings.NewReader(msg.content))

	// In order to deal with an existing buffer, we'll manually handle the
	// first line before looping over the rest of the scan.

	// If the write is just the empty string, there's nothing to do.
	if !scanner.Scan() {
		return
	}

	// If the first thing we scan is a newline, flush the buffer.
	// Otherwise, add it to the buffer and then flush.
	text := scanner.Text()
	m.lines, m.buffer = append(m.lines, m.buffer+text), ""

	// Now handle the rest of the lines.
	for scanner.Scan() {
		text := scanner.Text()
		m.lines = append(m.lines, text)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	// If the write didn't end with a newline, we overshot: the last line
	// we scanned should actually be the new buffer.
	if len(m.lines) > 0 && !strings.HasSuffix(msg.content, "\n") {
		m.buffer = m.lines[len(m.lines)-1]
		m.lines = m.lines[:len(m.lines)-1]
	}
}

func (m *Model) handleSearch(msg searchMsg) {
	m.query = msg.query
	results, err := m.search(m.query)
	if err == nil {
		m.results = results
		if len(results) != 0 {
			m.scrollPosition = results[0].line
		}
	}
}
