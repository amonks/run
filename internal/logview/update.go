package logview

import (
	"bufio"
	"regexp"
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
	case setFocusMsg:
		m.Focus = msg.focus
		if msg.focus == FocusSearchBar {
			m.handleSearch(m.SetQueryMsg("").(setQueryMsg))
		}
	case setDimensionsMsg:
		m.windowWidth, m.windowHeight = msg.width, msg.height
	case setQueryMsg:
		m.handleSearch(msg)
	case setResultIndexMsg:
		m.resultIndex = modulo(msg.index, len(m.results))
		if m.resultIndex < len(m.results) {
			result := m.results[m.resultIndex]
			m.scrollPosition = result.line
		}
	// case tea.WindowSizeMsg:
	// 	m.windowWidth, m.windowHeight = msg.Width, msg.Height
	case scrollByMsg:
		// if tailing, set scroll position to bottom
		if m.scrollPosition < 0 {
			m.scrollPosition = max(0, m.firstDisplayedLine)
			return m, nil
		}

		// update scroll position
		m.scrollPosition = max(0, m.scrollPosition+msg.lines)
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
	if m.Focus == FocusSearchBar {
		switch msg.String() {
		case "esc", "enter":
			return m.SetFocusMsg(FocusLogPane)
		case "backspace":
			if m.Query == "" {
				return m.SetFocusMsg(FocusLogPane)
			}
			return m.SetQueryMsg(m.Query[:len(m.Query)-1])
		default:
			return m.SetQueryMsg(m.Query + string(msg.Runes))
		}
	}
	switch msg.String() {
	case "/":
		return m.SetFocusMsg(FocusSearchBar)
	case "n":
		return m.SetResultIndexMsg(m.resultIndex + 1)
	case "N":
		return m.SetResultIndexMsg(modulo(m.resultIndex-1, len(m.results)))
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

	// Clear any search results in the buffer, since the buffer
	// will be different after this write.
	for i := len(m.results) - 1; i >= 0 && m.results[i].line < 0; i-- {
		m.results = m.results[:i]
	}

	// If the first thing we scan is a newline, flush the buffer.
	// Otherwise, add it to the buffer and then flush.
	text := scanner.Text()
	m.lines, m.buffer = append(m.lines, m.buffer+text), ""
	if results := m.searchLine(len(m.lines) - 1); len(results) > 0 {
		m.results = append(m.results, results...)
	}

	// Now handle the rest of the lines.
	for scanner.Scan() {
		text := scanner.Text()
		m.lines = append(m.lines, text)
		if results := m.searchLine(len(m.lines) - 1); len(results) > 0 {
			m.results = append(m.results, results...)
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	// If the write didn't end with a newline, we overshot: the last line
	// we scanned should actually be the new buffer.
	if len(m.lines) > 0 && !strings.HasSuffix(msg.content, "\n") {
		for i := len(m.results) - 1; i >= 0 && m.results[i].line == len(m.results)-1; i-- {
			m.results[i].line = -1
		}
		m.buffer = m.lines[len(m.lines)-1]
		m.lines = m.lines[:len(m.lines)-1]
	}
}

func (m *Model) handleSearch(msg setQueryMsg) {
	if msg.query == "" {
		m.Query = ""
		m.results = nil
		m.queryRe = nil
		return
	}

	m.Query = msg.query
	if queryRe, err := regexp.Compile(m.Query); err == nil {
		m.queryRe = queryRe
	}
	m.results = m.search()
	if len(m.results) != 0 {
		m.resultIndex = 0
		m.scrollPosition = m.results[0].line
	}
}

func modulo(i, n int) int {
	if n == 0 {
		return 0
	}
	return ((i % n) + n) % n
}
