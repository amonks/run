package help_test

import (
	"fmt"
	"testing"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/internal/help"
	"github.com/charmbracelet/lipgloss"
)

// no assertions; useful for testing visually; remove the Fail to see output
func TestRenderInline(t *testing.T) {
	for i := range 40 {
		w := i * 2
		fmt.Println("::::::")
		fmt.Println(s.Width(w).MaxWidth(w).Height(2).MaxHeight(2).Render(sec.RenderInline(help.Monochrome, w, 2)))
	}
	// t.Fail()
}

var s = lipgloss.NewStyle().Background(color.Green)

var sec = help.Section{
	Title: "Menu and Log View",
	Keys: []help.Key{
		{Keys: "?", Desc: "show help"},
		{Keys: "crtl+c", Desc: "quit"},

		{Keys: "enter or l", Desc: "select task"},
		{Keys: "esc or h", Desc: "deselect task"},
		{Keys: "tab", Desc: "select or deselect task"},
		{Keys: "0-9", Desc: "jump to task"},

		{Keys: "↑ or k", Desc: "up one line"},
		{Keys: "↓ or j", Desc: "down one line"},
		{Keys: "pgup", Desc: "up one page"},
		{Keys: "pgdown", Desc: "down one page"},
		{Keys: "ctrl+u", Desc: "up ½page"},
		{Keys: "ctrl+d", Desc: "down ½page"},
		{Keys: "home or gg", Desc: "go to top"},
		{Keys: "end or G", Desc: "go to tail"},

		{Keys: "/", Desc: "search task log"},
		{Keys: "N", Desc: "prev search result"},
		{Keys: "n", Desc: "next search result"},

		{Keys: "w", Desc: "toggle line wrapping"},
		{Keys: "s", Desc: "save task log to file"},
		{Keys: "r", Desc: "restart task"},
		{Keys: "c", Desc: "toggle dark mode"},
	},
}
