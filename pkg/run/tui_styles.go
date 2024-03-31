package run

import (
	"fmt"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/internal/help"
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

	menu, log                lipgloss.Style
	headerLeft, headerRight  lipgloss.Style
	footer                   lipgloss.Style
	lineStatus, searchStatus lipgloss.Style
}

var globalStyleCache = map[styleCacheKey]*styles{}

type styleCacheKey struct {
	width, height int
	focus         focusArea
}

func (m *tuiModel) styles(width, height int, focus focusArea) *styles {
	key := styleCacheKey{width, height, focus}
	if cached, isCached := globalStyleCache[key]; isCached {
		return cached
	}

	out := &styles{}
	globalStyleCache[key] = out

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
			out.menuWidth = min(width, 6)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				dotStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hash(id))).Underline(isSelected).Inline(true)
				return fmt.Sprintf("%s %2.1d %s", spinner, index, dotStyle.Render("•"))
			}
		} else if width < 256 {
			out.menuWidth = min(width, m.longestIDLength+11)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hash(id))).Underline(isSelected).Inline(true)
				return fmt.Sprintf("%s %s %2.1d %s %s", marker, spinner, index, taskStyle.Render("•"), taskStyle.Render(id))
			}
		} else {
			out.menuWidth = min(width, m.longestIDLength+19)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hash(id))).Underline(isSelected).Inline(true)
				return fmt.Sprintf("  %s %s %2.1d %s %s", marker, spinner, index, taskStyle.Render("•"), taskStyle.Render(id))
			}
		}
	case focusMenu:
		if width < 32 {
			out.menuWidth = width
			out.logWidth = 0
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hash(id))).Underline(isSelected).Inline(true)
				itemStyle := lipgloss.NewStyle().Inline(true).MaxWidth(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("%s %s %2.1d %s %s", marker, spinner, index, taskStyle.Render("•"), taskStyle.Render(id)))
			}
		} else if width < 256 {
			out.menuWidth = min(width, m.longestIDLength+11)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hash(id))).Underline(isSelected).Inline(true)
				itemStyle := lipgloss.NewStyle().Inline(true).MaxWidth(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("%s %s %2.1d %s %s", marker, spinner, index, taskStyle.Render("•"), taskStyle.Render(id)))
			}
		} else {
			out.menuWidth = min(width, m.longestIDLength+19)
			out.logWidth = max(0, width-out.menuWidth)
			out.renderMenuItem = func(id string, spinner string, marker string, index int, isSelected bool) string {
				taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color.Hash(id))).Underline(isSelected).Inline(true)
				itemStyle := lipgloss.NewStyle().Inline(true).MaxWidth(out.menuWidth)
				return itemStyle.Render(fmt.Sprintf("  %s %s %2.1d %s %s", marker, spinner, index, taskStyle.Render("•"), taskStyle.Render(id)))
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
			out.headerLine = hr(out.menuWidth, lipgloss.Color("#CCCCCC")) + hr(out.logWidth, lipgloss.Color("#FFFF00"))
		default:
			out.headerLine = hr(out.menuWidth, lipgloss.Color("#FFFF00")) + hr(out.logWidth, lipgloss.Color("#CCCCCC"))
		}
		out.menuHeight, out.logHeight = height-3, height-3
		out.headerLeft, out.headerRight = headerTall, headerTall
		out.footer = footerShort
	default:
		// big footer with help
		out.includeHeader, out.includeFooter = true, true
		switch focus {
		case focusLogs, focusSearch:
			out.headerLine = hr(out.menuWidth, lipgloss.Color("#CCCCCC")) + hr(out.logWidth, lipgloss.Color("#FFFF00"))
		default:
			out.headerLine = hr(out.menuWidth, lipgloss.Color("#FFFF00")) + hr(out.logWidth, lipgloss.Color("#CCCCCC"))
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
		Width(out.menuWidth).MaxWidth(out.menuWidth).
		Height(out.menuHeight).MaxHeight(out.menuHeight)
	out.log = lipgloss.NewStyle().
		Width(out.logWidth).MaxWidth(out.logWidth).
		Height(out.logHeight).MaxHeight(out.logHeight)
	out.headerLeft = out.headerLeft.Copy().
		Width(out.menuWidth).MaxWidth(out.menuWidth).
		Height(1).MaxHeight(1)
	out.headerRight = out.headerRight.Copy().
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
			Foreground(lipgloss.Color("#FFFF00")).
			Bold(true)
	headerShortActive = lipgloss.NewStyle().
				Padding(0, 2).
				Background(lipgloss.Color("#FFFF00")).
				Foreground(lipgloss.Color("#000000")).
				Bold(true).
				Underline(true)
	headerShortInactive = lipgloss.NewStyle().
				Padding(0, 2).
				Foreground(lipgloss.Color("#CCCCCC")).
				Background(lipgloss.Color("#000000"))
	footerShort = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("#CCCCCC")).
			Background(lipgloss.Color("#000000"))
	footerTall = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#000000"))

	inlineHelp = &help.Styles{
		Keys: lipgloss.NewStyle().
			Italic(true).
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#FFFFFF")),
		Desc: lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#AAAAAA")),
	}
)

func hr(width int, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("─", width))
}
