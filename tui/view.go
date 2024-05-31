package tui

import (
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/runner"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type uiZone = string

const (
	uiZoneLogs uiZone = "logs"
)

func (m *Model) View() string {
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

func (m *Model) renderHeader(styles *styles) string {
	var out strings.Builder
	rightStyle := styles.headerRight.Copy().Foreground(color.Hash(m.activeTaskID()))
	out.WriteString(styles.headerLeft.Render("") + rightStyle.Render(m.activeTaskID()))
	if styles.headerLine != "" {
		out.WriteString("\n" + styles.headerLine)
	}
	return out.String()
}

func (m *Model) renderMenu(styles *styles) string {
	status := m.tui.runner.Status()
	var out strings.Builder

	i := 0

	if len(status.MetaTasks) > 0 {
		out.WriteString(styles.menuHeader.Render("Meta Tasks") + "\n")
		for _, id := range status.MetaTasks {
			out.WriteString(m.renderMenuItem(styles, status, i, id) + "\n")
			i++
		}
	}

	if len(status.RequestedTasks) > 0 {
		out.WriteString(styles.menuHeader.Render("Requested Tasks") + "\n")
		for _, id := range status.RequestedTasks {
			out.WriteString(m.renderMenuItem(styles, status, i, id) + "\n")
			i++
		}
	}

	if len(status.ActiveTasks) > 0 {
		out.WriteString(styles.menuHeader.Render("Dependencies") + "\n")
		for _, id := range status.ActiveTasks {
			out.WriteString(m.renderMenuItem(styles, status, i, id) + "\n")
			i++
		}
	}

	if styles.includeInactiveTasks && len(status.InactiveTasks) > 0 {
		out.WriteString(styles.menuHeader.Render("Inactive Tasks") + "\n")
		for _, id := range status.InactiveTasks {
			out.WriteString(m.renderMenuItem(styles, status, i, id) + "\n")
			i++
		}
	}

	return styles.menu.Render(out.String())
}

func (m *Model) renderMenuItem(styles *styles, status runner.Status, index int, id string) string {
	taskStatus := status.TaskStatus[id]
	spinner := m.renderSpinner(taskStatus, id)
	marker, isSelected := " ", false
	if index == m.selectedTaskIDIndex {
		marker, isSelected = ">", true
	}
	return zone.Mark(id, styles.renderMenuItem(taskStatus, id, spinner, marker, index, isSelected))
}

func (m *Model) renderSpinner(taskStatus runner.TaskStatus, id string) string {
	if strings.HasPrefix(id, "@") {
		return " "
	}
	meta := m.tui.runner.Library().Task(id).Metadata()
	switch taskStatus {
	case runner.TaskStatusNotStarted:
		return " "
	case runner.TaskStatusRunning:
		if meta.Type == "long" {
			return m.longSpinner.View()
		} else {
			return m.shortSpinner.View()
		}
	case runner.TaskStatusRestarting:
		return m.shortSpinner.View()
	case runner.TaskStatusFailed:
		return "×"
	case runner.TaskStatusDone:
		return "✓"
	default:
		return " "
	}
}

func (m *Model) renderLog(styles *styles) string {
	activeLogview := m.tasks[m.activeTaskID()]
	return activeLogview.Render(styles.log, styles.logWidth, styles.logHeight)
}

func (m *Model) renderFooter(styles *styles) string {
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
