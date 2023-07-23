// Logview is a [bubbletea.Model] optimized for displaying logs.
package logview

import (
	"bufio"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

func New() *Model {
	return &Model{}
}

type Model struct {
	// TODO: incr
	id int

	width  int
	height int

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

func (m *Model) SetDimensions(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Scroll(n int) {
	m.scrollPosition = clamp(0, len(m.lines), n)
}

func (m *Model) ScrollToTop() {
	m.scrollPosition = 0
}

func (m *Model) ScrollToBottom() {
	m.scrollPosition = len(m.lines) - m.height
}

func (m *Model) ScrollTo(n int) {
	m.scrollPosition = n - (m.height / 2)
}

func (m *Model) WriteMsg(s string) tea.Msg {
	return writeMsg{
		id:      m.id,
		content: s,
	}
}

type writeMsg struct {
	id      int
	content string
}

func (m *Model) Write(s string) {
	scanner := bufio.NewScanner(strings.NewReader(s))

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
	if len(m.lines) > 0 && !strings.HasSuffix(s, "\n") {
		m.buffer = m.lines[len(m.lines)-1]
		m.lines = m.lines[:len(m.lines)-1]
	}
}

// Implement tea.Model
var _ tea.Model = &Model{}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case writeMsg:
		m.Write(msg.content)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	// linesOfInterest := m.displayedContent()
	linesOfInterest := m.content()
	content := strings.Join(linesOfInterest, "\n")
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Render(content)
}

func (m *Model) String() string {
	return strings.Join(m.content(), "\n")
}

// displayedContent returns up to [m.height] lines, beginning at
// m.scrollPosition.
//
// displayedContent does not account for line wrapping, but it's a suitably
// pared-down input to the line wrapper.
func (m *Model) displayedContent() []string {
	allLines := m.content()
	var linesOfInterest []string
	if m.scrollPosition < 0 {
		linesOfInterest = allLines[len(allLines)-m.height:]
	} else {
		linesOfInterest = allLines[m.scrollPosition:min(len(allLines), m.scrollPosition+m.height)]
	}
	return linesOfInterest
}

func (m *Model) content() []string {
	lines := m.lines
	if m.buffer != "" {
		lines = append(m.lines, m.buffer)
	}
	return lines
}

func (m *Model) search(query string) ([]searchResult, error) {
	var results []searchResult
	re, err := regexp.Compile(query)
	if err != nil {
		return nil, err
	}
	for i, l := range m.lines {
		for _, m := range re.FindAllStringIndex(l, -1) {
			results = append(results, searchResult{
				line:   i,
				char:   m[0],
				length: m[1] - m[0],
			})
		}
	}
	if m.buffer != "" {
		for _, m := range re.FindAllStringIndex(m.buffer, -1) {
			results = append(results, searchResult{
				line:   -1,
				char:   m[0],
				length: m[1] - m[0],
			})
		}
	}
	return results, nil
}

type searchResult struct {
	// 0-indexed line number where match appears. Negative for a match in
	// the buffer.
	line int

	// 0-indexed column number of the rune that begins the match.
	char int

	// Length of the match.
	length int
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(min, max, n int) int {
	if n < min {
		return min
	} else if n > max {
		return max
	} else {
		return n
	}
}

func hardwrap(s string, width int) string {
	var b strings.Builder
	for _, l := range strings.Split(s, "\n") {
		b.WriteString(truncate.String(l, uint(width)) + "\n")
	}
	return b.String()
}
