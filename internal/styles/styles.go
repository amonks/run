package styles

import (
	"github.com/amonks/run/internal/color"
	"github.com/charmbracelet/lipgloss"
)

var (
	Log = lipgloss.NewStyle().
		Foreground(color.XXXLight).
		Italic(true)
)
