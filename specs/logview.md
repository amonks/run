# Log View

## Overview

The `logview` package (`pkg/logview`) provides a standalone BubbleTea model for viewing and searching logs. It is used by the TUI's task log panes and by the `scroll` command.

## Construction

```go
func New(mods ...func(*Model)) *Model
```

Functional options:
- `WithoutStatusbar`: hides the status bar.
- `WithStartAtHead`: starts scrolled to the top instead of tailing.
- `WithHardWrap`: enables hard-wrapping of long lines.

## Scroll Behavior

- `scrollPosition` tracks viewport position:
  - Negative: tailing mode (viewport follows new content).
  - Non-negative: pinned mode (the Nth line is at the top of the viewport).
- `ScrollBy(lines int)`: adjusts scroll position. If tailing, first pins to the current bottom.
- `ScrollTo(line int)`: jumps to a specific line. Negative values re-enter tailing mode.

## Content Model

- `lines []string`: completed lines (terminated by `\n`).
- `buffer string`: incomplete content since the last `\n`.
- `Write(content string)`: appends text, splitting on newlines.
- `String() string`: returns all content joined by newlines.

## Search

- `SetQuery(query string)`: sets the search input and triggers search.
- Regex-based search across all lines.
- `MoveResultIndex(by int)`: cycles through results (wrapping).
- `SetResultIndex(index int)`: jumps to a specific result.
- Scrolls to the line containing the current result.
- Result count and index shown in the status bar.

## Focus Areas

```go
const (
    FocusSearchBar FocusArea
    FocusLogPane   FocusArea
    FocusHelp      FocusArea
)
```

- `SetFocus(focus FocusArea)`: switches focus. Activates search input when focusing the search bar.

## Rendering

- `View() string`: standard BubbleTea rendering.
- `Render(styles *Styles, width, height int) string`: custom rendering with external styles (used by the TUI to control layout).
- Status bar shows line position and search state.
- Wrap mode is toggleable at runtime with `SetWrapMode(bool)` or `ToggleWrapMode()`.

## BubbleTea Integration

Implements `tea.Model`:
- `Init() tea.Cmd`: returns nil.
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)`: handles keyboard, mouse, and window events.
- Supports vim-style navigation (`j/k`, `gg`, `G`, `Ctrl+D/U`).

## Additional Methods

- `SetDimensions(width, height int)`: sets viewport size.
- `ShowStatusbar(bool)`: toggles status bar visibility.
- `Query() string`: returns current search query.
- `Results() []searchResult`: returns search results.
- `Focus() FocusArea`: returns current focus area.
- `RenderLineStatus() string` and `RenderSearchStatus() string`: for external status bar rendering.
