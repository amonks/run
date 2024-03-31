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
