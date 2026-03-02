package runner

import (
	"github.com/amonks/run/internal/color"
	"charm.land/lipgloss/v2"
)

// LogStyle is the style used for run system messages (starting, exit, etc.).
var LogStyle = lipgloss.NewStyle().
	Foreground(color.XXXLight).
	Italic(true)

// logStyle is an alias for internal use.
var logStyle = LogStyle
