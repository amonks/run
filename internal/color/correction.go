package color

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderWithCorrection(style lipgloss.Style, s string) string {
	const ansiEscape = "\033[0m"
	var ansiSetBackground, ansiSetForeground string
	{
		r, g, b := extract(style.GetBackground())
		if r == 0 && g == 0 && b == 0 {
			return style.Render(s)
		}
		ansiSetBackground = fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
	}
	{
		r, g, b := extract(style.GetForeground())
		ansiSetForeground = fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	}
	s = strings.ReplaceAll(s, ansiEscape, ansiEscape+ansiSetBackground+ansiSetForeground)
	return style.Render(s)
}

func extract(c lipgloss.TerminalColor) (int, int, int) {
	r, g, b, a := c.RGBA()
	rf, gf, bf, af := float64(r), float64(g), float64(b), float64(a)
	return int(rf / af * 255), int(gf / af * 255), int(bf / af * 255)
}
