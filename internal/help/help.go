package help

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type Menu []Section

type Section struct {
	Title string
	Keys  []Key
}

type Key struct {
	Keys string
	Desc string
}

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

var (
	Monochrome = &Styles{
		Container: lipgloss.NewStyle(),
		Header:    lipgloss.NewStyle().Transform(strings.ToUpper),
		Keys:      lipgloss.NewStyle().Bold(true),
		Desc:      lipgloss.NewStyle().Italic(true),
	}
	Colored = &Styles{
		Container: lipgloss.NewStyle().
			Padding(2, 4),
		Header: lipgloss.NewStyle().
			Underline(true).
			Bold(true).
			MarginBottom(1).
			Foreground(lipgloss.Color("#FFFF00")).
			Background(lipgloss.Color("#000000")),
		Keys: lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#999999")).
			Background(lipgloss.Color("#000000")),
		Desc: lipgloss.NewStyle().Italic(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#000000")),
	}
)

type Styles struct {
	Container lipgloss.Style
	Header    lipgloss.Style
	Keys      lipgloss.Style
	Desc      lipgloss.Style
}

func (m Menu) Render(styles *Styles, width, height int) string {
	var out strings.Builder
	var longest int
	for _, section := range m {
		for _, k := range section.Keys {
			if l := lipgloss.Width(k.Keys); l > longest {
				longest = l
			}
		}
	}
	for _, section := range m {
		out.WriteString(styles.Header.Render(section.Title) + "\n")
		for _, k := range section.Keys {
			pad := strings.Repeat(" ", longest-lipgloss.Width(k.Keys))
			out.WriteString(fmt.Sprintf("  %s%s %s\n", styles.Keys.Render(k.Keys), pad, styles.Desc.Render(k.Desc)))
		}
		out.WriteString("\n")
	}
	return styles.Container.Render(out.String())
}

func (s Section) Render(styles *Styles, width, height int) string {
	var out strings.Builder
	i := 0
	for range height {
		lineLength := 0
		for ; i < len(s.Keys); i++ {
			k := s.Keys[i]
			rendered := fmt.Sprintf("%s: %s", styles.Keys.Render(k.Keys), styles.Desc.Render(k.Desc))
			if lineLength+lipgloss.Width(rendered)+4 > width {
				break
			}
			lineLength += lipgloss.Width(rendered) + 4
			out.WriteString(rendered + "    ")
		}
		out.WriteString("\n")
	}
	return out.String()
}
