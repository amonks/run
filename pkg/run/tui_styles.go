package run

import (
	"fmt"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/internal/help"
	"github.com/amonks/run/pkg/logview"
	"github.com/charmbracelet/lipgloss"
)

type styles struct {
	menuWidth, menuHeight             int
	logWidth, logHeight               int
	inlineHelpWidth, inlineHelpHeight int
	inlineHelp                        *help.Styles

	renderMenuItem func(id, spinner, marker string, index int, isSelected bool) string

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
	darkMode      bool
	width, height int
	focus         focusArea
}

func (m *tuiModel) styles(width, height int, focus focusArea) *styles {
	key := styleCacheKey{lipgloss.HasDarkBackground(), width, height, focus}
	if cached, isCached := globalStyleCache[key]; isCached {
		return cached
	}

	out := &styles{}
	globalStyleCache[key] = out

	menuBackground, logBackground := color.XXXDark, color.XXDark
	if focus == focusMenu {
		menuBackground, logBackground = logBackground, menuBackground
	}

	// set width stuff
	switch focus {
	case focusLogs, focusSearch:
		if width < 64 {
			out.menuWidth = 0
			out.logWidth = width
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				return ""
			}
		} else if width < 128 {
			out.menuWidth = min(width, 8)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				dotStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.Hash(id)).Underline(isSelected).Inline(true)
				return fmt.Sprintf("%s %2.1d %s", spinner, index, dotStyle.Render("•"))
			}
		} else if width < 256 {
			out.menuWidth = min(width, m.longestIDLength+11)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.Hash(id)).Inline(true)
				return fmt.Sprintf("%s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id))
			}
		} else {
			out.menuWidth = min(width, m.longestIDLength+19)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.Hash(id)).Inline(true)
				return fmt.Sprintf("  %s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id))
			}
		}
	case focusMenu:
		if width < 32 {
			out.menuWidth = width
			out.logWidth = 0
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.Hash(id)).Inline(true)
				itemStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.XXXLight).Inline(true).MaxWidth(out.menuWidth).Width(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("%s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id)))
			}
		} else if width < 256 {
			out.menuWidth = min(width, m.longestIDLength+11)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.Hash(id)).Inline(true)
				itemStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.XXXLight).Inline(true).MaxWidth(out.menuWidth).Width(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("%s %s %2.1d %s", marker, spinner, index, taskStyle.Render("• "+id)))
			}
		} else {
			out.menuWidth = min(width, m.longestIDLength+19)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.Hash(id)).Inline(true)
				itemStyle := lipgloss.NewStyle().Background(menuBackground).Foreground(color.XXXLight).Inline(true).MaxWidth(out.menuWidth).Width(out.menuWidth)
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
		out.inlineHelpWidth, out.inlineHelpHeight = width, 2
		out.menuHeight, out.logHeight = height-6, height-6
		out.headerLeft, out.headerRight = headerTall, headerTall
		out.footer = footerTall
	}

	// these are just derived from the dimensions
	out.menu = lipgloss.NewStyle().
		Background(menuBackground).
		Width(out.menuWidth).MaxWidth(out.menuWidth).
		Height(out.menuHeight).MaxHeight(out.menuHeight)
	out.log = &logview.Styles{
		Log: lipgloss.NewStyle().
			Background(logBackground).
			Foreground(color.XXXLight).
			Width(out.logWidth).MaxWidth(out.logWidth).
			Height(out.logHeight).MaxHeight(out.logHeight),
	}
	out.headerLeft = out.headerLeft.Copy().
		Background(color.XXDark).
		Width(out.menuWidth).MaxWidth(out.menuWidth).
		Height(1).MaxHeight(1)
	out.headerRight = out.headerRight.Copy().
		Background(color.XXDark).
		Width(out.logWidth).MaxWidth(out.logWidth).
		Height(1).MaxHeight(1)
	out.footer = out.footer.Width(width).MaxWidth(width).Copy().
		Height(1).MaxHeight(1 + out.inlineHelpHeight)

	// these are staic
	out.inlineHelp = inlineHelp

	return out
}

var (
	headerTall = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(color.Yellow).
			Background(color.XXDark).
			Bold(true)
	headerShortActive = lipgloss.NewStyle().
				Padding(0, 2).
				Background(color.Yellow).
				Foreground(color.XXDark).
				Bold(true).
				Underline(true)
	headerShortInactive = lipgloss.NewStyle().
				Padding(0, 2).
				Background(color.XXDark).
				Foreground(color.XXLight)
	footerShort = lipgloss.NewStyle().
			Padding(0, 2).
			Background(color.XXDark).
			Foreground(color.XXLight)
	footerTall = lipgloss.NewStyle().
			Padding(0, 2).
			Background(color.XXDark).
			Foreground(color.XXLight)

	inlineHelp = &help.Styles{
		Keys: lipgloss.NewStyle().
			Italic(true).
			Background(color.XXDark).
			Foreground(color.XXLight),
		Desc: lipgloss.NewStyle().
			Background(color.XXDark).
			Foreground(color.XLight),
	}
)

func hr(width int, emphasize bool) string {
	if emphasize {
		return lipgloss.NewStyle().Background(color.XXDark).Foreground(color.Yellow).Render(strings.Repeat("─", width))
	} else {
		return lipgloss.NewStyle().Background(color.XXXDark).Foreground(color.XDark).Render(strings.Repeat("─", width))
	}
}
