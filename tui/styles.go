package tui

import (
	"fmt"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/internal/help"
	"github.com/amonks/run/logview"
	"github.com/amonks/run/runner"
	"github.com/charmbracelet/lipgloss"
)

type styles struct {
	menuWidth, menuHeight             int
	logWidth, logHeight               int
	inlineHelpWidth, inlineHelpHeight int
	inlineHelp                        *help.Styles

	menuHeader           lipgloss.Style
	includeInactiveTasks bool
	visibleMenuItems     []string
	renderMenuItem       func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string

	includeHeader, includeFooter, includeInlineHelp bool
	headerLine, footerLine                          string

	menu                     lipgloss.Style
	log                      *logview.Styles
	headerLeft, headerRight  lipgloss.Style
	footer                   lipgloss.Style
	lineStatus, searchStatus lipgloss.Style
}

var globalStyleCache = map[styleCacheKey]*styles{}

type styleCacheKey struct {
	width, height int
	focus         focusArea
}

func (m *Model) styles(width, height int, focus focusArea) *styles {
	key := styleCacheKey{width, height, focus}
	if cached, isCached := globalStyleCache[key]; isCached {
		return cached
	}

	out := &styles{}
	globalStyleCache[key] = out

	// Start adding visible menu items. Later on, we'll optionally add
	// InactiveTasks based on the height of the viewport.
	status := m.tui.runner.Status()
	for _, id := range status.MetaTasks {
		out.visibleMenuItems = append(out.visibleMenuItems, id)
	}
	for _, id := range status.RequestedTasks {
		out.visibleMenuItems = append(out.visibleMenuItems, id)
	}
	for _, id := range status.ActiveTasks {
		out.visibleMenuItems = append(out.visibleMenuItems, id)
	}

	// set width stuff
	switch focus {
	case focusLogs, focusSearch:
		if width < 64 {
			out.menuWidth = 0
			out.logWidth = width
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				return ""
			}
		} else if width < 128 {
			out.menuWidth = min(width, 8)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				dotStyle := lipgloss.NewStyle().Foreground(color.Hash(id)).Underline(isSelected).Inline(true)
				return fmt.Sprintf("%s %2.1d %s", spinner, index, dotStyle.Render("•"))
			}
		} else if width < 256 {
			out.menuWidth = min(width, m.longestIDLength+11)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(color.Hash(id)).Inline(true)
				return fmt.Sprintf("%s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id))
			}
		} else {
			out.menuWidth = min(width, m.longestIDLength+19)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(color.Hash(id)).Inline(true)
				return fmt.Sprintf("  %s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id))
			}
		}
	case focusMenu:
		if width < 32 {
			out.menuWidth = width
			out.logWidth = 0
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(color.Hash(id)).Inline(true)
				itemStyle := lipgloss.NewStyle().Foreground(color.XXXLight).Inline(true).MaxWidth(out.menuWidth).Width(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("%s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id)))
			}
		} else if width < 256 {
			out.menuWidth = min(width, m.longestIDLength+11)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(color.Hash(id)).Inline(true)
				itemStyle := lipgloss.NewStyle().Foreground(color.XXXLight).Inline(true).MaxWidth(out.menuWidth).Width(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("%s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id)))
			}
		} else {
			out.menuWidth = min(width, m.longestIDLength+19)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(taskStatus runner.TaskStatus, id, spinner, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(color.Hash(id)).Inline(true)
				itemStyle := lipgloss.NewStyle().Foreground(color.XXXLight).Inline(true).MaxWidth(out.menuWidth).Width(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("  %s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id)))
			}
		}
	}

	// set height stuff
	switch true {
	case height <= 2:
		// no header or footer
		out.menuHeight, out.logHeight = height, height
	case height <= 4:
		// add footer
		out.includeFooter = true
		out.menuHeight, out.logHeight = height-1, height-1
		out.footer = footerShort
	case height < 12:
		// add header
		out.includeHeader, out.includeFooter = true, true
		out.menuHeight, out.logHeight = height-2, height-2
		out.headerLeft, out.headerRight = headerShortActive, headerShortInactive
		switch focus {
		case focusLogs, focusSearch:
			out.headerLeft, out.headerRight = headerShortInactive, headerShortActive
		}
		out.footer = footerShort
	case height < 24:
		// big header
		out.includeHeader, out.includeFooter = true, true
		switch focus {
		case focusLogs, focusSearch:
			out.headerLine = hr(out.menuWidth, false) + hr(out.logWidth, true)
		default:
			out.headerLine = hr(out.menuWidth, true) + hr(out.logWidth, false)
		}
		out.menuHeight, out.logHeight = height-3, height-3
		out.headerLeft, out.headerRight = headerTall, headerTall
		out.footer = footerShort
	default:
		// big footer with help
		out.includeHeader, out.includeFooter = true, true
		switch focus {
		case focusLogs, focusSearch:
			out.headerLine = hr(out.menuWidth, false) + hr(out.logWidth, true)
		default:
			out.headerLine = hr(out.menuWidth, true) + hr(out.logWidth, false)
		}
		out.footerLine = out.headerLine
		out.includeInlineHelp = true
		out.inlineHelpWidth, out.inlineHelpHeight = width-4, 2
		out.menuHeight, out.logHeight = height-6, height-6
		out.headerLeft, out.headerRight = headerTall, headerTall
		out.footer = footerTall

		out.includeInactiveTasks = true
		for _, id := range status.InactiveTasks {
			out.visibleMenuItems = append(out.visibleMenuItems, id)
		}
	}

	// Set other values that derive from dimensions.
	out.menu = lipgloss.NewStyle().
		Width(out.menuWidth).MaxWidth(out.menuWidth).
		Height(out.menuHeight).MaxHeight(out.menuHeight)
	out.log = &logview.Styles{
		Log: lipgloss.NewStyle().
			Foreground(color.XXXLight).
			Width(out.logWidth).MaxWidth(out.logWidth).
			Height(out.logHeight).MaxHeight(out.logHeight),
	}
	out.headerLeft = out.headerLeft.Copy().
		Width(out.menuWidth).MaxWidth(out.menuWidth).
		Height(1).MaxHeight(1)
	out.headerRight = out.headerRight.Copy().
		Width(out.logWidth).MaxWidth(out.logWidth).
		Height(1).MaxHeight(1)
	out.footer = out.footer.Width(width).MaxWidth(width).Copy().
		Height(1).MaxHeight(1 + out.inlineHelpHeight)

	// Set static values.
	out.inlineHelp = inlineHelp

	return out
}

var (
	logStyle = lipgloss.NewStyle().
			Foreground(color.XXXLight).
			Italic(true)

	headerTall = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(color.Yellow).
			Bold(true)
	headerShortActive = lipgloss.NewStyle().
				Padding(0, 2).
				Background(color.Light).
				Foreground(color.XXXDark).
				Bold(true).
				Underline(true)
	headerShortInactive = lipgloss.NewStyle().
				Padding(0, 2).
				Foreground(color.XXLight)
	footerShort = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(color.XXLight)
	footerTall = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(color.XXLight)

	inlineHelp = &help.Styles{
		Keys: lipgloss.NewStyle().
			Italic(true).
			Foreground(color.XXLight),
		Desc: lipgloss.NewStyle().
			Foreground(color.XLight),
	}
)

func hr(width int, emphasize bool) string {
	if emphasize {
		return lipgloss.NewStyle().Foreground(color.Yellow).Render(strings.Repeat("─", width))
	} else {
		return lipgloss.NewStyle().Foreground(color.XDark).Render(strings.Repeat("─", width))
	}
}
