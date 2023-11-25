package logview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wrap"
)

func (m *Model) View() string {
	if m.windowWidth <= 0 || m.windowHeight <= 0 {
		return ""
	}
	content := m.displayLog()

	return lipgloss.NewStyle().
		Width(m.windowWidth).Height(m.windowHeight).
		Render(content + "\n" + m.statusBar())
}

func (m *Model) statusBar() string {
	var out string
	linecount := len(m.lines)
	if m.buffer != "" {
		linecount += 1
	}

	if m.scrollPosition < 0 {
		out = fmt.Sprintf("tail (%d lines)", linecount)
	} else {
		out = fmt.Sprintf("line %d of %d", m.scrollPosition, linecount)
	}

	if m.Query != "" {
		out += fmt.Sprintf("\tquery: '%s'", m.Query)
	}
	if len(m.results) != 0 {
		out += fmt.Sprintf("\tresult %d of %d", m.resultIndex+1, len(m.results))
	}

	return out
}

func (m *Model) displayLog() string {
	// If we're tailing, start assembling output from the -end- of the log,
	// returning it when we have enough
	if m.scrollPosition < 0 {
		var (
			linecount     = len(m.lines)
			pointer       = linecount - 1
			searchPointer = len(m.results) - 1
			output        = ""
			outputHeight  = 0
			targetHeight  = m.logHeight()
		)

		// handle the buffer, if present
		if m.buffer != "" {
			fmt.Println("has buffer")
			if searchPointer < len(m.results) && searchPointer >= 0 && m.results[searchPointer].line == -1 {
				m.buffer = strings.ReplaceAll(m.buffer, m.Query, highlight.Render(m.Query))
				for searchPointer >= 0 && m.results[searchPointer].line == -1 {
					searchPointer -= 1
				}
			}
			wrapped, wrappedHeight := m.wrapLine(m.buffer, targetHeight)
			output = "\n" + wrapped
			outputHeight = wrappedHeight
		}

		for ; outputHeight < targetHeight && pointer >= 0; pointer-- {
			l := m.lines[pointer]
			if searchPointer < len(m.results) && searchPointer >= 0 && m.results[searchPointer].line == pointer {
				l = strings.ReplaceAll(l, m.Query, highlight.Render(m.Query))
				for searchPointer >= 0 && m.results[searchPointer].line == pointer {
					searchPointer -= 1
				}
			}
			wrapped, wrappedHeight := m.wrapLine(l, targetHeight-outputHeight)
			output = "\n" + wrapped + output
			outputHeight += wrappedHeight
		}
		m.firstDisplayedLine = pointer

		output = strings.TrimPrefix(output, "\n")

		if outputHeight < targetHeight {
			pad := strings.Repeat(" ", m.windowWidth) + "\n"
			for outputHeight < targetHeight {
				output = pad + output
				outputHeight += 1
			}
		}

		return output
	}

	// If we're not tailing, start from m.scrollPosition and keep adding
	// wrapped output until we reach m.logHeight
	var (
		linecount     = len(m.lines)
		pointer       = m.scrollPosition
		searchPointer = 0
		output        = ""
		outputHeight  = 0
		targetHeight  = m.logHeight()
	)

	for searchPointer < len(m.results)-1 && m.results[searchPointer].line < m.scrollPosition {
		searchPointer += 1
	}

	m.firstDisplayedLine = m.scrollPosition

	// handle the lines
	for ; outputHeight < targetHeight && pointer < linecount; pointer++ {
		l := m.lines[pointer]
		if searchPointer < len(m.results) && m.results[searchPointer].line == pointer {
			l = strings.ReplaceAll(l, m.Query, highlight.Render(m.Query))
			for searchPointer < len(m.results) && m.results[searchPointer].line == pointer {
				searchPointer += 1
			}
		}
		wrapped, wrappedHeight := m.wrapLine(l, targetHeight-outputHeight)
		output = output + wrapped + "\n"
		outputHeight += wrappedHeight
	}

	// handle the buffer
	if outputHeight < targetHeight && m.buffer != "" {
		l := m.buffer
		if searchPointer >= 0 && searchPointer < len(m.results) && m.results[searchPointer].line == -1 {
			l = strings.ReplaceAll(l, m.Query, highlight.Render(m.Query))
			for searchPointer < len(m.results) && m.results[searchPointer].line == -1 {
				searchPointer += 1
			}
		}
		wrapped, wrappedHeight := m.wrapLine(l, targetHeight-outputHeight)
		output = output + wrapped + "\n"
		outputHeight += wrappedHeight
	}

	output = strings.TrimSuffix(output, "\n")

	return output
}

func (m *Model) wrapLine(line string, maxLines int) (string, int) {
	if m.shouldHardwrap {
		wrapped := truncate.String(line, uint(m.windowWidth))
		return wrapped, 1
	} else {
		wrapped := wrap.String(line, m.windowWidth)
		wrappedHeight := strings.Count(wrapped, "\n") + 1
		if wrappedHeight > maxLines {
			wrappedHeight = maxLines
			if m.scrollPosition < 0 {
				wrapped = lastNLines(wrapped, wrappedHeight)
			} else {
				wrapped = firstNLines(wrapped, wrappedHeight)
			}
		}
		return wrapped, wrappedHeight
	}
}

func firstNLines(s string, n int) string {
	return strings.Join(strings.Split(s, "\n")[:n], "\n")
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	return strings.Join(lines[len(lines)-n:], "\n")
}

var highlight = lipgloss.NewStyle().
	Background(lipgloss.Color("#FFFF00")).
	Foreground(lipgloss.Color("#000000"))
