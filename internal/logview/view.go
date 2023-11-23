package logview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

func (m *Model) View() string {
	linesOfInterest := m.displayedContent()
	content := strings.Join(linesOfInterest, "\n")

	if m.shouldHardwrap {
		content = hardwrap(content, m.windowWidth)
	}

	return lipgloss.NewStyle().
		Width(m.windowWidth).Height(m.logHeight()).
		Render(content) + fmt.Sprintf("\n%s : %d", m.query, m.scrollPosition)
}

// displayedContent returns up to [m.height] lines, beginning at
// m.scrollPosition.
//
// displayedContent does not account for line wrapping, but it's a suitably
// pared-down input to the line wrapper.
func (m *Model) displayedContent() []string {
	allLines := m.content()
	linecount := len(allLines)
	logheight := m.logHeight()

	if linecount < logheight {
		return allLines
	}

	// if tailing, return last m.logHeight lines
	if m.scrollPosition < 0 {
		return allLines[max(linecount-logheight, 0):]
	}

	// return m.logHeight lines beginning at m.scrollPosition
	return allLines[m.scrollPosition:min(len(allLines), m.scrollPosition+m.logHeight())]
}

func hardwrap(s string, width int) string {
	var b strings.Builder
	lines := strings.Split(s, "\n")
	first, rest := lines[0], lines[1:]
	b.WriteString(truncate.String(first, uint(width)))
	for _, l := range rest {
		b.WriteString("\n" + truncate.String(l, uint(width)))
	}
	return b.String()
}
