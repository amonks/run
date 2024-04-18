package color

import "github.com/charmbracelet/lipgloss"

func NewTrueColor(hex string) lipgloss.CompleteColor {
	return lipgloss.CompleteColor{
		TrueColor: hex,
		ANSI:      "30",
		ANSI256:   "30",
	}
}

func NewAdaptiveTrueColor(dark, light string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Light: NewTrueColor(light),
		Dark:  NewTrueColor(dark),
	}
}
