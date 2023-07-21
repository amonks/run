package run

import "github.com/charmbracelet/lipgloss"

var (
	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCC")).
			Italic(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F00")).
			Italic(true)
)


var (
	listStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			BorderStyle(lipgloss.HiddenBorder()).
			Margin(0).Padding(0)
	listItemStyle = lipgloss.NewStyle().
			Padding(0)
	previewStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			BorderStyle(lipgloss.NormalBorder()).
			Margin(0).Padding(0, 1, 1, 2)
	pagerStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			Margin(0).Padding(0)
	helpStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			Foreground(lipgloss.Color("#CCC")).
			Italic(true).
			Margin(0).Padding(0)
)
