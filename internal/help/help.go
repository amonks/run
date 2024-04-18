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

	rendered []renderedInlineHelpItem
}

type renderedInlineHelpItem struct {
	width int
	str   string
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
			MarginBottom(1),
		Keys: lipgloss.NewStyle().Bold(true),
		Desc: lipgloss.NewStyle().Italic(true),
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

func (s *Section) renderInlineHelpItems(styles *Styles) {
	if s.rendered != nil {
		return
	}
	s.rendered = make([]renderedInlineHelpItem, len(s.Keys))
	for i, k := range s.Keys {
		s.rendered[i] = renderedInlineHelpItem{}
		s.rendered[i].str = styles.Keys.Render(k.Keys) + styles.Desc.Render(": "+k.Desc)
		s.rendered[i].width = lipgloss.Width(s.rendered[i].str)
	}
}

func (s *Section) RenderInline(styles *Styles, width, height int) string {
	s.renderInlineHelpItems(styles)
	var out strings.Builder
	i := 0
	for range height {
		out.WriteString("\n")
		lineLength := 0
		first := s.rendered[i]
		if lineLength+first.width > width {
			continue
		}
		out.WriteString(first.str)
		lineLength += first.width
		i++

		for ; i < len(s.rendered); i++ {
			item := s.rendered[i]
			itemLabel := "    " + item.str
			itemWidth := 4 + item.width

			if lineLength+itemWidth > width {
				// fmt.Printf("break at %d of %d: lineLength: %d; itemWidth: %d; width: %d; itemLabel: '%s'\n", i, len(s.rendered), lineLength, itemWidth, width, itemLabel)
				break
			}
			out.WriteString(itemLabel)
			lineLength += itemWidth
		}
	}
	return out.String()[1:]
}
