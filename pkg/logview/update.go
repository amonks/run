package logview

import (
	"bufio"
	"regexp"
	"strings"

	help "github.com/amonks/run/internal/help"
	tea "github.com/charmbracelet/bubbletea/v2"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := m.handleKey(msg)
		return m, cmd
	case tea.MouseMsg:
		m.handleMouse(msg)
	default:
		newInput, cmd := m.input.Update(msg)
		m.input = &newInput
		return m, cmd
	}
	return m, nil
}

var helpmenu = help.Menu{
	{
		Title: "Logview",
		Keys: []help.Key{
			{Keys: "crtl+c", Desc: "quit"},
			{Keys: "?, h", Desc: "show help"},

			{Keys: "/", Desc: "search"},
			{Keys: "N", Desc: "prev search result"},
			{Keys: "n", Desc: "next search result"},

			{Keys: "↑ or k", Desc: "up one line"},
			{Keys: "↓ or j", Desc: "down one line"},
			{Keys: "pgup", Desc: "up one page"},
			{Keys: "pgdown", Desc: "down one page"},
			{Keys: "ctrl+u", Desc: "up ½page"},
			{Keys: "ctrl+d", Desc: "down ½page"},
			{Keys: "home or gg", Desc: "go to top"},
			{Keys: "end or G", Desc: "follow tail"},
		},
	},
	{
		Title: "Search",
		Keys: []help.Key{
			{Keys: "enter", Desc: "search"},
			{Keys: "esc", Desc: "cancel"},
		},
	},
	{
		Title: "Help",
		Keys: []help.Key{
			{Keys: "esc", Desc: "exit help"},
		},
	},
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.focus == FocusHelp {
		switch msg.String() {
		case "esc", "ctrl+c", "q", "h", "?":
			m.SetFocus(FocusLogPane)
		}
		return nil
	}
	if m.focus == FocusSearchBar {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.input.SetValue(m.prevQuery)
			m.prevQuery = ""
			m.SetFocus(FocusLogPane)
		case "enter":
			m.SetFocus(FocusLogPane)
			m.resultInStatusbar = true
		case "backspace":
			if m.Query() == "" {
				m.SetFocus(FocusLogPane)
				return nil
			}
			fallthrough
		default:
			queryBefore := m.input.Value()
			newSearch, cmd := m.input.Update(msg)
			m.input = &newSearch
			if newSearch.Value() != queryBefore {
				m.handleSearch()
			}
			return cmd
		}
		return nil
	}

	if k := msg.String(); k != "n" && k != "N" {
		m.resultInStatusbar = false
	}

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return tea.Quit
	case "h", "?":
		m.SetFocus(FocusHelp)
	case "w":
		m.SetWrapMode(!m.shouldHardwrap)
	case "/":
		m.prevQuery = m.Query()
		m.SetQuery("")
		m.SetFocus(FocusSearchBar)
	case "n":
		m.MoveResultIndex(1)
	case "N":
		m.MoveResultIndex(-1)

	case "up", "k":
		m.ScrollBy(-1)
	case "down", "j":
		m.ScrollBy(1)

	case "pgup":
		pageDistance := max(0, m.windowHeight-4)
		m.ScrollBy(-pageDistance)
	case "pgdown":
		pageDistance := max(0, m.windowHeight-4)
		m.ScrollBy(pageDistance)

	case "ctrl+u":
		m.ScrollBy(-m.windowHeight / 2)
	case "ctrl+d":
		m.ScrollBy(m.windowHeight / 2)

	case "home":
		m.ScrollTo(0)
	case "end":
		m.ScrollTo(-1)

	case "g":
		if m.heldKey == "g" {
			m.ScrollTo(0)
			m.heldKey = ""
		} else {
			m.heldKey = "g"
		}
	case "G":
		m.ScrollTo(-1)
	}
	return nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) {
	switch msg.Mouse().Button {
	case tea.MouseWheelDown:
		m.ScrollBy(1)
	case tea.MouseWheelUp:
		m.ScrollBy(-1)
	}
}

func (m *Model) handleWrite(content string) {
	scanner := bufio.NewScanner(strings.NewReader(content))

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
	if len(m.lines) > 0 && !strings.HasSuffix(content, "\n") {
		for i := len(m.results) - 1; i >= 0 && m.results[i].line == len(m.results)-1; i-- {
			m.results[i].line = -1
		}
		m.buffer = m.lines[len(m.lines)-1]
		m.lines = m.lines[:len(m.lines)-1]
	}
}

func (m *Model) handleSearch() {
	query := m.input.Value()

	if query == "" {
		m.results = nil
		m.queryRe = nil
		return
	}

	if queryRe, err := regexp.Compile(query); err == nil {
		m.queryRe = queryRe
	}
	m.results = m.search()
	if len(m.results) != 0 {
		m.resultIndex = 0
		m.scrollPosition = m.results[0].line
	}
}
