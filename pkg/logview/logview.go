// Logview is a [bubbletea.Model] optimized for displaying logs.
package logview

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func New(mods ...func(*Model)) *Model {
	inp := textinput.New()
	inp.Prompt = "/"

	m := &Model{
		scrollPosition:      -1,
		shouldShowStatusbar: true,
		input:               &inp,
	}
	for _, mod := range mods {
		mod(m)
	}
	return m
}

func WithoutStatusbar(m *Model) { m.shouldShowStatusbar = false }
func WithStartAtHead(m *Model)  { m.scrollPosition = 0 }
func WithHardWrap(m *Model)     { m.shouldHardwrap = true }

// [Model] implements [tea.Model]
var _ tea.Model = &Model{}

type Model struct {
	windowWidth  int
	windowHeight int

	shouldHardwrap      bool
	shouldShowStatusbar bool

	focus FocusArea

	input     *textinput.Model
	queryRe   *regexp.Regexp
	prevQuery string

	results           []searchResult
	resultInStatusbar bool
	resultIndex       int

	// state for two-key inputs like `gg`
	heldKey string

	// ScrollPosition tracks the position of the viewport relative to the
	// log's content.
	//  - If it's negative, we are tailing the log.
	//  - If it's positive, the scrollPositionth line is pinned to the top
	//    of the viewport.
	// For example, if scrollPosition is 0, the 0th line is displayed at
	// the top of the viewport.
	scrollPosition int

	firstDisplayedLine int

	// lines contains all complete lines (that is, a "\n" was written to
	// end the line).
	lines []string

	// If the most recent character written was not a "\n", buffer contains
	// everything that was written since the last "\n".
	buffer string
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) String() string {
	return strings.Join(m.content(), "\n")
}

func (m *Model) Write(content string) { m.handleWrite(content) }

func (m *Model) SetDimensions(width, height int) { m.windowWidth, m.windowHeight = width, height }

func (m *Model) Query() string {
	return m.input.Value()
}

func (m *Model) Results() []searchResult {
	return m.results
}

func (m *Model) ResultIndex() int {
	return m.resultIndex
}

func (m *Model) Focus() FocusArea {
	return m.focus
}

func (m *Model) SetFocus(focus FocusArea) {
	m.focus = focus
	switch focus {
	case FocusSearchBar:
		m.input.Focus()
		m.handleSearch()
	default:
		m.input.Blur()
	}
}

func (m *Model) SetQuery(query string) {
	m.input.SetValue(query)
	m.handleSearch()
}

func (m *Model) MoveResultIndex(by int) {
	m.resultInStatusbar = true
	m.resultIndex = modulo(m.resultIndex+by, len(m.results))
	if m.resultIndex < len(m.results) {
		result := m.results[m.resultIndex]
		m.scrollPosition = result.line
	}
}

func (m *Model) SetResultIndex(index int) {
	m.resultInStatusbar = true
	m.resultIndex = modulo(index, len(m.results))
	if m.resultIndex < len(m.results) {
		result := m.results[m.resultIndex]
		m.scrollPosition = result.line
	}
}

func (m *Model) ScrollBy(lines int) {
	// if tailing, set scroll position to bottom
	if m.scrollPosition < 0 {
		m.scrollPosition = max(0, m.firstDisplayedLine)
		return
	}

	// update scroll position
	m.scrollPosition = clamp(0, len(m.lines)-1, m.scrollPosition+lines)
}

func (m *Model) ScrollTo(line int) {
	if line < 0 {
		m.scrollPosition = -1
	} else {
		m.scrollPosition = clamp(0, len(m.lines)-1, line)
	}
}

func (m *Model) ShowStatusbar(show bool) { m.shouldShowStatusbar = show }

func (m *Model) SetWrapMode(hardwrap bool) { m.shouldHardwrap = hardwrap }
func (m *Model) ToggleWrapMode()           { m.shouldHardwrap = !m.shouldHardwrap }

func (m *Model) logHeight(height int) int {
	if !m.shouldShowStatusbar {
		return height
	}
	return max(height-1, 0)
}

func (m *Model) content() []string {
	lines := m.lines
	if m.buffer != "" {
		lines = append(m.lines, m.buffer)
	}
	return lines
}

type FocusArea int

const (
	focusNone FocusArea = iota
	FocusSearchBar
	FocusLogPane
	FocusHelp
)
