package color

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

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

	XXXLight = compat.AdaptiveColor{Dark: lipgloss.Color("#FDF6E3"), Light: lipgloss.Color("#002B36")} // base3
	XXLight  = compat.AdaptiveColor{Dark: lipgloss.Color("#EEE8D5"), Light: lipgloss.Color("#073642")} // base2
	XLight   = compat.AdaptiveColor{Dark: lipgloss.Color("#93A1A1"), Light: lipgloss.Color("#586E75")} // base1
	Light    = compat.AdaptiveColor{Dark: lipgloss.Color("#839496"), Light: lipgloss.Color("#657B83")} // base0
	Dark     = compat.AdaptiveColor{Dark: lipgloss.Color("#657B83"), Light: lipgloss.Color("#839496")} // base00
	XDark    = compat.AdaptiveColor{Dark: lipgloss.Color("#586E75"), Light: lipgloss.Color("#93A1A1")} // base01
	XXDark   = compat.AdaptiveColor{Dark: lipgloss.Color("#073642"), Light: lipgloss.Color("#EEE8D5")} // base02
	XXXDark  = compat.AdaptiveColor{Dark: lipgloss.Color("#002B36"), Light: lipgloss.Color("#FDF6E3")} // base03
)
