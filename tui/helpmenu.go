package tui

import "github.com/amonks/run/internal/help"

var helpMenu = help.Menu{
	{
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
		},
	},
	{
		Title: "Search",
		Keys: []help.Key{
			{Keys: "enter", Desc: "search"},
			{Keys: "esc", Desc: "cancel"},
			{Keys: "crtl+c", Desc: "quit"},
		},
	},
	{
		Title: "Help",
		Keys: []help.Key{
			{Keys: "esc or q", Desc: "exit help"},
			{Keys: "crtl+c", Desc: "quit"},
		},
	},
}
