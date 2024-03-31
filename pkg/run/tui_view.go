package run

import (
	"fmt"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/pkg/logview"
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
	rightStyle := styles.headerRight.Copy().Foreground(lipgloss.Color(color.Hash(m.activeTaskID())))
	out.WriteString(styles.headerLeft.Render("") + rightStyle.Render(m.activeTaskID()))
	if styles.headerLine != "" {
		out.WriteString("\n" + styles.headerLine)
	}
	return out.String()
}

func (m *tuiModel) renderMenu(styles *styles) string {
	var out strings.Builder
	for i, id := range m.ids {
		out.WriteString(m.renderMenuItem(styles, i, id) + "\n")
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
	if id == "interleaved" || id == "run" {
		return " "
	}
	if _, has := m.tui.run.Tasks()[id]; !has {
		panic(id)
	}
	var (
		meta   = m.tui.run.Tasks()[id].Metadata()
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
	return activeLogview.Render(styles.logWidth, styles.logHeight)
}

func (m *tuiModel) renderFooter(styles *styles) string {
	var out strings.Builder
	if styles.footerLine != "" {
		out.WriteString(styles.footerLine + "\n")
	}

	var footer strings.Builder

	footer.WriteString(styles.lineStatus.Render(m.activeTask().RenderLineStatus()))
	footer.WriteString("\t")
	footer.WriteString(styles.searchStatus.Render(m.renderSearch()))

	if styles.includeInlineHelp {
		footer.WriteString("\n" + helpMenu[0].Render(styles.inlineHelp, styles.inlineHelpWidth, styles.inlineHelpHeight))
	}

	out.WriteString(styles.footer.Render(footer.String()))

	return out.String()
}

func (m *tuiModel) renderSearch() string {
	task := m.activeTask()
	query := task.Query()
	results := task.Results()
	focus := task.Focus()
	resultIndex := task.ResultIndex()
	var out strings.Builder
	if query != "" || focus == logview.FocusSearchBar {
		out.WriteString(fmt.Sprintf("/%s", query))
	}
	if len(results) != 0 {
		out.WriteString(fmt.Sprintf(": %d of %d", resultIndex+1, len(results)))
	}
	return out.String()
}
