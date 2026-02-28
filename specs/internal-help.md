# Internal Help

## Overview

The `internal/help` package renders help menus for the TUI, supporting both full overlay and inline footer layouts.

## Types

```go
type Menu []Section

type Section struct {
    Title string
    Keys  []Key
}

type Key struct {
    Keys string  // e.g., "Ctrl+C"
    Desc string  // e.g., "Quit"
}
```

## Rendering

### Full Overlay

`Menu.Render(styles *Styles, width, height int) string`:
- Renders all sections vertically.
- Key names are left-aligned to the longest key across all sections.
- Each section has a styled header followed by indented key-description pairs.

### Inline Footer

`Section.RenderInline(styles *Styles, width, height int) string`:
- Renders a single section's keys horizontally, wrapping across available lines.
- Items are separated by 4 spaces.
- Items that exceed the remaining line width wrap to the next line.
- Rendered items are cached on the section for reuse.

## Styles

Two predefined style sets:

- `Monochrome`: uppercase headers, bold keys, italic descriptions. No color.
- `Colored`: underlined bold headers with Solarized yellow, bold keys on light background, italic descriptions. Container has padding.
