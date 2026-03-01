package run

import (
	"github.com/amonks/run/internal/color"
	"charm.land/lipgloss/v2"
)

var (
	logStyle = lipgloss.NewStyle().
			Foreground(color.XXXLight).
			Italic(true)
)
