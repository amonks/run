package color

import "github.com/charmbracelet/lipgloss"

// https://ethanschoonover.com/solarized/#the-values
var (
	Yellow  = lipgloss.Color("#B58900")
	Orange  = lipgloss.Color("#CB4B16")
	Red     = lipgloss.Color("#DC322F")
	Magenta = lipgloss.Color("#D33682")
	Violet  = lipgloss.Color("#6C71C4")
	Blue    = lipgloss.Color("#268BD2")
	Cyan    = lipgloss.Color("#2AA198")
	Green   = lipgloss.Color("#859900")

	XXXLight = lipgloss.AdaptiveColor{Dark: "#FDF6E3", Light: "#002B36"} // base3
	XXLight  = lipgloss.AdaptiveColor{Dark: "#EEE8D5", Light: "#073642"} // base2
	XLight   = lipgloss.AdaptiveColor{Dark: "#93A1A1", Light: "#586E75"} // base1
	Light    = lipgloss.AdaptiveColor{Dark: "#839496", Light: "#657B83"} // base0
	Dark     = lipgloss.AdaptiveColor{Dark: "#657B83", Light: "#839496"} // base00
	XDark    = lipgloss.AdaptiveColor{Dark: "#586E75", Light: "#93A1A1"} // base01
	XXDark   = lipgloss.AdaptiveColor{Dark: "#073642", Light: "#EEE8D5"} // base02
	XXXDark  = lipgloss.AdaptiveColor{Dark: "#002B36", Light: "#FDF6E3"} // base03
)
