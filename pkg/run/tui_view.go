package run

import (
	"fmt"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type uiZone = string

const (
	uiZoneLogs uiZone = "logs"
)

func (m *tuiModel) View() string {
	if !m.didInit || !m.gotSize {
		return ""
	}

	if m.focus == focusHelp {
		return m.help.View()
	}

	styles := m.styles(m.width, m.height, m.focus)

	var sections []string
	if styles.includeHeader {
		sections = append(sections, m.renderHeader(styles))
	}
	sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderMenu(styles),
		zone.Mark(uiZoneLogs, m.renderLog(styles)),
	))
	if styles.includeFooter {
		sections = append(sections, m.renderFooter(styles))
	}
	return zone.Scan(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)
}

func (m *tuiModel) renderHeader(styles *styles) string {
	var out strings.Builder
	rightStyle := styles.headerRight.Foreground(color.Hash(m.activeTaskID()))
	out.WriteString(styles.headerLeft.Render("") + rightStyle.Render(m.activeTaskID()))
	if styles.headerLine != "" {
		out.WriteString("\n" + styles.headerLine)
	}
	return out.String()
}

func (m *tuiModel) renderMenu(styles *styles) string {
	total := len(m.ids)
	height := styles.menuHeight
	selected := m.selectedTaskIDIndex

	// If all tasks fit, render as-is.
	if total <= height {
		var out strings.Builder
		for i, id := range m.ids {
			out.WriteString(m.renderMenuItem(styles, i, id) + "\n")
		}
		return styles.menu.Render(out.String())
	}

	// Not enough room for indicators; just window around the selection.
	if height < 3 {
		start := selected - height/2
		if start < 0 {
			start = 0
		}
		end := start + height
		if end > total {
			end = total
			start = max(0, end-height)
		}
		var out strings.Builder
		for i := start; i < end; i++ {
			out.WriteString(m.renderMenuItem(styles, i, m.ids[i]) + "\n")
		}
		return styles.menu.Render(out.String())
	}

	// Scrolling with indicators.
	offset := m.menuScrollOffset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		offset = total - 1
	}

	// Iteratively adjust offset so the selected task is visible,
	// accounting for indicator lines consuming menu height.
	for iter := 0; iter < 3; iter++ {
		showUp := offset > 0
		taskSlots := height
		if showUp {
			taskSlots--
		}
		showDown := offset+taskSlots < total
		if showDown {
			taskSlots--
		}
		if selected < offset {
			offset = selected
			continue
		}
		if selected >= offset+taskSlots {
			offset = selected - taskSlots + 1
			continue
		}
		break
	}

	// Final visible window computation.
	showUp := offset > 0
	taskSlots := height
	if showUp {
		taskSlots--
	}
	end := min(offset+taskSlots, total)
	showDown := end < total
	if showDown {
		taskSlots--
		end = min(offset+taskSlots, total)
	}

	m.menuScrollOffset = offset

	indicatorStyle := lipgloss.NewStyle().Foreground(color.XDark)

	var out strings.Builder
	if showUp {
		out.WriteString(indicatorStyle.Render(fmt.Sprintf("▲ %d", offset)) + "\n")
	}
	for i := offset; i < end; i++ {
		out.WriteString(m.renderMenuItem(styles, i, m.ids[i]) + "\n")
	}
	if showDown {
		out.WriteString(indicatorStyle.Render(fmt.Sprintf("▼ %d", total-end)) + "\n")
	}
	return styles.menu.Render(out.String())
}

func (m *tuiModel) renderMenuItem(styles *styles, index int, id string) string {
	spinner := m.renderSpinner(id)
	marker, isSelected := " ", false
	if index == m.selectedTaskIDIndex {
		marker, isSelected = ">", true
	}
	return zone.Mark(id, styles.renderMenuItem(id, spinner, marker, index, isSelected))
}

func (m *tuiModel) renderSpinner(id string) string {
	if strings.HasPrefix(id, "@") {
		return " "
	}
	var (
		meta   = m.tui.run.Tasks().Get(id).Metadata()
		status = m.tui.run.TaskStatus(id)
	)
	switch status {
	case TaskStatusNotStarted:
		return " "
	case TaskStatusRunning:
		if meta.Type == "long" {
			return m.longSpinner.View()
		} else {
			return m.shortSpinner.View()
		}
	case TaskStatusRestarting:
		return m.shortSpinner.View()
	case TaskStatusFailed:
		return "×"
	case TaskStatusDone:
		return "✓"
	default:
		return "?"
	}
}

func (m *tuiModel) renderLog(styles *styles) string {
	activeLogview := m.tasks[m.activeTaskID()]
	return activeLogview.Render(styles.log, styles.logWidth, styles.logHeight)
}

func (m *tuiModel) renderFooter(styles *styles) string {
	var out strings.Builder
	if styles.footerLine != "" {
		out.WriteString(styles.footerLine + "\n")
	}

	var footer strings.Builder

	if m.quitKey != "" {
		footer.WriteString("press " + m.quitKey + " again to quit")
	} else {
		footer.WriteString(styles.lineStatus.Render(m.activeTask().RenderLineStatus()))
		footer.WriteString("\t")
		footer.WriteString(styles.searchStatus.Render(m.activeTask().RenderSearchStatus()))
	}

	if styles.includeInlineHelp {
		footer.WriteString("\n" + helpMenu[0].RenderInline(styles.inlineHelp, styles.inlineHelpWidth, styles.inlineHelpHeight))
	}

	out.WriteString(styles.footer.Render(footer.String()))

	return out.String()
}
