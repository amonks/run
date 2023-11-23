// Logview is a [bubbletea.Model] optimized for displaying logs.
package logview

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var ids = &incr{}

func New() *Model {
	return &Model{
		id:                   ids.incr(),
		scrollPosition:       -1,
		shouldHardwrap:       true,
		shouldHandleKeyboard: true,
		shouldHandleMouse:    true,
	}
}

// [Model] implements [tea.Model]
var _ tea.Model = &Model{}

type Model struct {
	id int

	windowWidth  int
	windowHeight int

	query   string
	results []searchResult

	shouldHandleKeyboard bool
	shouldHandleMouse    bool
	shouldHardwrap       bool

	// ScrollPosition tracks the position of the viewport relative to the
	// log's content.
	//  - If it's negative, we are tailing the log.
	//  - If it's positive, the scrollPositionth line is pinned to the top
	//    of the viewport.
	// For example, if scrollPosition is 0, the 0th line is displayed at
	// the top of the viewport.
	scrollPosition int

	// lines contains all complete lines (that is, a "\n" was written to
	// end the line).
	lines []string

	// If the most recent character written was not a "\n", buffer contains
	// everything that was written since the last "\n".
	buffer string
}

func (m *Model) logHeight() int {
	return max(m.windowHeight - 1, 0)
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) String() string {
	return strings.Join(m.content(), "\n")
}

func (m *Model) content() []string {
	lines := m.lines
	if m.buffer != "" {
		lines = append(m.lines, m.buffer)
	}
	return lines
}
