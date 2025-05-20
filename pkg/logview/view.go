package logview

import (
	"fmt"
	"strings"

	help "github.com/amonks/run/internal/help"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wrap"
)

type Styles struct {
	Log       lipgloss.Style
	Statusbar lipgloss.Style
}

var defaultStyles = &Styles{
	Log:       lipgloss.NewStyle(),
	Statusbar: lipgloss.NewStyle(),
}

func (m *Model) View() string {
	return m.Render(defaultStyles, m.windowWidth, m.windowHeight)
}

func (m *Model) Render(styles *Styles, width, height int) string {
	// don't crash if window has zero area
	if width <= 0 || height <= 0 {
		return ""
	}

	// show help instead of content in help mode
	if m.focus == FocusHelp {
		return helpmenu.Render(help.Monochrome, width, height)
	}

	// skip statusbar if window is too short
	if height < 2 || !m.shouldShowStatusbar {
		content := m.RenderLog(width, height)
		logStyle := styles.Log.
			Width(width).Height(height).
			MaxWidth(width).MaxHeight(height)
		return logStyle.Render(content)
	}

	// render logview and statusbar
	content := m.RenderLog(width, height-1)
	logStyle := styles.Log.
		Width(width).Height(height - 1).
		MaxWidth(width).MaxHeight(height - 1)
	logview := logStyle.Render(content)
	statusbar := styles.Statusbar.
		Width(width).Height(1).
		MaxWidth(width).MaxHeight(1).
		Render(m.viewStatusbar())
	return logview + "\n" + statusbar
}

func (m *Model) viewStatusbar() string {
	return m.RenderLineStatus() + "\t" + m.RenderSearchStatus()
}

func (m *Model) RenderLineStatus() string {
	linecount := len(m.lines)
	if m.buffer != "" {
		linecount += 1
	}

	if m.scrollPosition < 0 {
		return fmt.Sprintf("tail of %d", linecount)
	}
	return fmt.Sprintf("%d of %d", m.scrollPosition+1, linecount)
}

func (m *Model) RenderSearchStatus() string {
	var out string
	if m.Query() != "" || m.focus == FocusSearchBar {
		out += m.input.View()
	}
	if len(m.results) != 0 && (m.focus == FocusSearchBar || m.resultInStatusbar) {
		out += fmt.Sprintf(": %d of %d", m.resultIndex+1, len(m.results))
	}
	return out
}

func (m *Model) RenderLog(width, height int) string {
	// If we're tailing, start assembling output from the -end- of the log,
	// returning it when we have enough
	if m.scrollPosition < 0 {
		var (
			linecount     = len(m.lines)
			pointer       = linecount - 1
			searchPointer = len(m.results) - 1
			output        = ""
			outputHeight  = 0
			targetHeight  = height
		)

		// handle the buffer, if present
		if m.buffer != "" {
			if searchPointer < len(m.results) && searchPointer >= 0 && m.results[searchPointer].line == -1 {
				m.buffer = strings.ReplaceAll(m.buffer, m.Query(), highlight.Render(m.Query()))
				for searchPointer >= 0 && m.results[searchPointer].line == -1 {
					searchPointer -= 1
				}
			}
			wrapped, wrappedHeight := m.wrapLine(m.buffer, targetHeight, width)
			output = "\n" + wrapped
			outputHeight = wrappedHeight
		}

		for ; outputHeight < targetHeight && pointer >= 0; pointer-- {
			l := m.lines[pointer]
			if searchPointer < len(m.results) && searchPointer >= 0 && m.results[searchPointer].line == pointer {
				l = strings.ReplaceAll(l, m.Query(), highlight.Render(m.Query()))
				for searchPointer >= 0 && m.results[searchPointer].line == pointer {
					searchPointer -= 1
				}
			}
			wrapped, wrappedHeight := m.wrapLine(l, targetHeight-outputHeight, width)
			output = "\n" + wrapped + output
			outputHeight += wrappedHeight
		}
		m.firstDisplayedLine = pointer

		output = strings.TrimPrefix(output, "\n")

		if outputHeight < targetHeight {
			pad := strings.Repeat(" ", width) + "\n"
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
		targetHeight  = m.logHeight(height)
	)

	for searchPointer < len(m.results)-1 && m.results[searchPointer].line < m.scrollPosition {
		searchPointer += 1
	}

	m.firstDisplayedLine = m.scrollPosition

	// handle the lines
	for ; outputHeight < targetHeight && pointer < linecount; pointer++ {
		l := m.lines[pointer]
		if searchPointer < len(m.results) && m.results[searchPointer].line == pointer {
			l = string(m.queryRe.ReplaceAllFunc([]byte(l), func(bs []byte) []byte {
				return []byte(highlight.Render(string(bs)))
			}))
			for searchPointer < len(m.results) && m.results[searchPointer].line == pointer {
				searchPointer += 1
			}
		}
		wrapped, wrappedHeight := m.wrapLine(l, targetHeight-outputHeight, width)
		output = output + wrapped + "\n"
		outputHeight += wrappedHeight
	}

	// handle the buffer
	if outputHeight < targetHeight && m.buffer != "" {
		l := m.buffer
		if searchPointer >= 0 && searchPointer < len(m.results) && m.results[searchPointer].line == -1 {
			l = strings.ReplaceAll(l, m.Query(), highlight.Render(m.Query()))
			for searchPointer < len(m.results) && m.results[searchPointer].line == -1 {
				searchPointer += 1
			}
		}
		wrapped, wrappedHeight := m.wrapLine(l, targetHeight-outputHeight, width)
		output = output + wrapped + "\n"
		outputHeight += wrappedHeight
	}

	return strings.TrimSuffix(output, "\n")
}

func (m *Model) wrapLine(line string, maxLines, width int) (string, int) {
	if m.shouldHardwrap {
		wrapped := truncate.String(line, uint(width))
		return wrapped, 1
	} else {
		wrapped := wrap.String(line, width)
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
	lines := strings.Split(s, "\n")
	return strings.Join(lines[:min(n, len(lines))], "\n")
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

var highlight = lipgloss.NewStyle().
	Background(lipgloss.Color("#FFFF00")).
	Foreground(lipgloss.Color("#000000"))
