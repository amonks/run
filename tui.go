package run

import (
	"fmt"
	"io"
	"strconv"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/reflow/wordwrap"
)

func newTUI() UI {
	zone.NewGlobal()
	return &tui{mu: newMutex("tui")}
}

type tui struct {
	mu *mutex

	// nil until started
	p *tea.Program

	interleaved UI

	waiters []chan<- error
}

// *tui implements MultiWriter
var _ MultiWriter = &tui{}

func (a *tui) Writer(id string) io.Writer {
	defer a.mu.Lock("Writer").Unlock()

	if a.p == nil {
		panic("getting writer from unstarted tui")
	}

	var send func(tea.Msg)
	if a.p != nil {
		send = a.p.Send
	}

	return tuiWriter{
		mu:                newMutex("writer:" + id),
		id:                id,
		interleavedWriter: a.interleaved,
		send:              send,
	}
}

// *writer implements io.Writer
var _ io.Writer = tuiWriter{}

type tuiWriter struct {
	mu                *mutex
	id                string
	interleavedWriter MultiWriter
	send              func(tea.Msg)
}

func (w tuiWriter) Write(bs []byte) (int, error) {
	defer w.mu.Lock("Write").Unlock()

	if w.send == nil {
		panic("nil send")
	}
	if w.id != "interleaved" {
		if w.interleavedWriter == nil {
			panic("nil interleaved writer")
		}
		w.interleavedWriter.Writer(w.id).Write(bs)
	}
	w.send(writeMsg{key: w.id, content: string(bs)})
	return len(bs), nil
}

// *tui implements UI
var _ UI = &tui{}

func (a *tui) Start(stdin io.Reader, stdout io.Writer, ids []string) error {
	a.mu.Lock("Start")

	// already started
	if a.p != nil {
		a.mu.Unlock()
		return nil
	}

	ready := make(chan struct{})
	a.p = tea.NewProgram(
		&tuiModel{ids: append([]string{"interleaved"}, ids...), onInit: func() { ready <- struct{}{}; close(ready) }},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	a.mu.Unlock()
	interleavedWriter := a.Writer("interleaved")
	defer a.mu.Lock("start 2").Unlock()

	p := NewPrinter()
	p.Start(nil, interleavedWriter, ids)
	a.interleaved = p

	go func() {
		// run the bubbletea Program
		_, err := a.p.Run()

		// When it exits, notify Waiters that the UI is done.
		a.notify(err)
	}()

	<-ready

	return nil
}

func (a *tui) Wait() <-chan error {
	defer a.mu.Lock("Wait").Unlock()

	c := make(chan error)
	a.waiters = append(a.waiters, c)
	return c
}

func (a *tui) notify(err error) {
	defer a.mu.Lock("notify").Unlock()

	for _, w := range a.waiters {
		select {
		case w <- err:
		default:
		}
		close(w)
	}
}

func (a *tui) Stop() error {
	return nil
}

type tuiModel struct {
	onInit func()

	ids       []string
	listWidth int

	width  int
	height int

	lastkey string
	didInit bool
	gotSize bool

	isPaging   bool
	activeTask string

	tasks map[string]string

	help        help.Model
	list        list.Model
	shortOutput viewport.Model
	preview     viewport.Model
	pager       viewport.Model
}

func (m *tuiModel) Init() tea.Cmd {
	fmt.Fprintln(logfile, "init")
	m.preview = viewport.New(0, 0)
	m.pager = viewport.New(0, 0)

	longestKey := 0
	items := make([]list.Item, len(m.ids))
	for i, id := range m.ids {
		if len(id) > longestKey {
			longestKey = len(id)
		}
		items[i] = listItem(id)
	}
	m.listWidth = longestKey + 8
	if len(m.ids) > 9 {
		m.listWidth = longestKey + 9
	}
	m.list = list.New(items, itemDelegate{m}, 0, 0)
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
	m.list.SetShowHelp(false)
	m.list.SetShowTitle(false)

	m.help.ShowAll = true
	m.help.ShortSeparator = "   "
	m.help.FullSeparator = "     "

	m.didInit = true

	return func() tea.Msg { return initilaizedMsg{} }
}

type initilaizedMsg struct{}

type writeMsg struct {
	key     string
	content string
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case initilaizedMsg:
		m.onInit()

	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			for i, id := range m.list.Items() {
				if zone.Get(string(id.(listItem))).InBounds(msg) {
					m.list.Select(i)
				}
			}
		} else {
			cmds = append(cmds, m.passthrough(msg)...)
		}

	case tea.KeyMsg:
		if !m.didInit || !m.gotSize {
			return m, nil
		}
		if m.isPaging {
			switch true {
			case msg.String() == "g":
				if m.lastkey == "g" {
					m.pager.GotoTop()
				}
			case key.Matches(msg, pagerKeymap.bottom):
				m.pager.GotoBottom()
			case key.Matches(msg, pagerKeymap.up):
				m.pager.LineUp(1)
			case key.Matches(msg, pagerKeymap.down):
				m.pager.LineDown(1)
			case key.Matches(msg, pagerKeymap.exit):
				m.isPaging = false
			}
		} else {
			switch true {
			case msg.String() == "g":
				if m.lastkey == "g" {
					m.list.Select(0)
				}
			case key.Matches(msg, pagerKeymap.bottom):
				m.list.Select(len(m.tasks) - 1)
			case key.Matches(msg, listKeymap.jump):
				n, err := strconv.Atoi(msg.String())
				if err != nil {
					panic(err)
				}
				i := n - 1
				if i < len(m.ids) {
					m.list.Select(i)
				}
			case key.Matches(msg, listKeymap.down):
				m.list.CursorDown()
			case key.Matches(msg, listKeymap.up):
				m.list.CursorUp()
			case key.Matches(msg, listKeymap.open):
				m.isPaging = true
				m.updatePager()
				m.pager.GotoTop()
			case key.Matches(msg, listKeymap.exit):
				return m, tea.Quit
			}
		}
		m.lastkey = msg.String()

	case writeMsg:
		if m.tasks == nil {
			m.tasks = map[string]string{}
		}
		m.tasks[msg.key] += msg.content
		if msg.key == "interleaved" {
			wasAtBottom := m.pager.AtBottom()
			m.updateShortOutput()
			if m.didInit && m.gotSize && wasAtBottom {
				m.shortOutput.GotoBottom()
			}
		}
		if m.activeTask == msg.key {
			if m.isPaging {
				wasAtBottom := m.pager.AtBottom()
				m.updatePager()
				if m.didInit && m.gotSize && wasAtBottom {
					m.pager.GotoBottom()
				}
			} else {
				m.updatePreview()
				if m.didInit && m.gotSize {
					m.preview.GotoBottom()
				}
			}
		}

	case tea.WindowSizeMsg:
		// model
		m.width = msg.Width
		m.height = msg.Height

		// short output
		m.shortOutput.Width = msg.Width
		m.shortOutput.Height = msg.Height

		// help
		helpStyle = helpStyle.
			UnsetMaxWidth().MaxWidth(m.width).
			UnsetMaxHeight().MaxHeight(helpHeight)
		helpStyle = helpStyle.
			UnsetWidth().Width(helpStyle.GetMaxWidth() - helpStyle.GetHorizontalFrameSize()).
			UnsetHeight().Height(helpStyle.GetMaxHeight() - helpStyle.GetVerticalFrameSize())

		// pager
		pagerStyle = pagerStyle.
			UnsetMaxWidth().MaxWidth(m.width).
			UnsetMaxHeight().MaxHeight(m.height - helpHeight)
		pagerStyle = pagerStyle.
			UnsetWidth().Width(pagerStyle.GetMaxWidth() - pagerStyle.GetHorizontalFrameSize()).
			UnsetHeight().Height(pagerStyle.GetMaxHeight() - pagerStyle.GetVerticalFrameSize())
		m.pager.Width = pagerStyle.GetMaxWidth() - pagerStyle.GetHorizontalFrameSize()
		m.pager.Height = pagerStyle.GetMaxHeight() - pagerStyle.GetVerticalFrameSize()

		// list
		listStyle = listStyle.
			UnsetMaxWidth().MaxWidth(m.listWidth).
			UnsetMaxHeight().MaxHeight(m.height - helpHeight)
		listStyle = listStyle.
			UnsetWidth().Width(listStyle.GetMaxWidth() - listStyle.GetHorizontalFrameSize()).
			UnsetHeight().Height(listStyle.GetMaxHeight() - listStyle.GetVerticalFrameSize())
		m.list.SetSize(
			listStyle.GetMaxWidth()-listStyle.GetHorizontalFrameSize(),
			listStyle.GetMaxHeight()-listStyle.GetVerticalFrameSize())

		// preview
		previewStyle = previewStyle.
			UnsetMaxWidth().MaxWidth(m.width - m.listWidth).
			UnsetMaxHeight().MaxHeight(m.height - helpHeight)
		previewStyle = previewStyle.
			UnsetWidth().Width(previewStyle.GetMaxWidth() - previewStyle.GetHorizontalFrameSize()).
			UnsetHeight().Height(previewStyle.GetMaxHeight() - previewStyle.GetVerticalFrameSize())
		m.preview.Width = previewStyle.GetMaxWidth() - previewStyle.GetHorizontalFrameSize()
		m.preview.Height = previewStyle.GetMaxHeight() - previewStyle.GetVerticalFrameSize()

		// done
		m.gotSize = true

		m.updatePager()
		m.updatePreview()
		m.updateShortOutput()

	default:
		cmds = append(cmds, m.passthrough(msg)...)
	}

	if item := m.list.SelectedItem(); item != nil {
		selectedItem := string(item.(listItem))
		if selectedItem != m.activeTask {
			m.activeTask = selectedItem
			m.updatePreview()
			if m.didInit && m.gotSize {
				m.preview.GotoBottom()
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) passthrough(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	shortOutput, cmd := m.shortOutput.Update(msg)
	cmds = append(cmds, cmd)
	m.shortOutput = shortOutput

	newPager, cmd := m.pager.Update(msg)
	cmds = append(cmds, cmd)
	m.pager = newPager

	newViewport, cmd := m.preview.Update(msg)
	cmds = append(cmds, cmd)
	m.preview = newViewport

	newList, cmd := m.list.Update(msg)
	cmds = append(cmds, cmd)
	m.list = newList

	return cmds
}

func (m *tuiModel) updatePreview() {
	m.preview.SetContent(wordwrap.String(m.tasks[m.activeTask], previewStyle.GetWidth()-previewStyle.GetHorizontalFrameSize()))
}
func (m *tuiModel) updatePager() {
	m.pager.SetContent(wordwrap.String(m.tasks[m.activeTask], pagerStyle.GetWidth()-pagerStyle.GetHorizontalFrameSize()))
}
func (m *tuiModel) updateShortOutput() {
	m.shortOutput.SetContent(wordwrap.String(m.tasks["interleaved"], m.width))
}

func (m *tuiModel) View() string {
	if !m.didInit || !m.gotSize {
		return "......."
	}

	if m.height <= 14 {
		return m.shortOutput.View()
	}

	if m.isPaging {
		return zone.Scan(lipgloss.JoinVertical(
			lipgloss.Left,
			pagerStyle.Render(m.pager.View()),
			helpStyle.Render(m.help.View(pagerKeymap)),
		))
	}

	return zone.Scan(lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			listStyle.Render(m.list.View()),
			previewStyle.Render(m.preview.View()),
		),
		helpStyle.Render(m.help.View(listKeymap)),
	))

}

type listItem string

func (i listItem) Title() string       { return string(i) }
func (i listItem) FilterValue() string { return string(i) }

const (
	helpHeight = 3
)

type itemDelegate struct{ m *tuiModel }

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(listItem)
	if !ok {
		return
	}
	id := string(i)

	style := itemStyle.Copy()
	var str string
	if index == m.Index() {
		style = style.Foreground(lipgloss.Color("#F0F"))
		str = fmt.Sprintf("> %d. %s", index+1, id)
	} else {
		str = fmt.Sprintf("  %d. %s", index+1, id)
	}

	if len(d.m.tasks[id]) == 0 {
		style = style.Italic(true)
	}

	fmt.Fprint(w, zone.Mark(id, style.Render(str)))
}

var (
	debugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCC"))
	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCC")).
			Italic(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F00")).
			Italic(true)

	itemStyle = lipgloss.NewStyle().
			Padding(0)

	listStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			BorderStyle(lipgloss.HiddenBorder()).
			Margin(0).Padding(0)
	previewStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			BorderStyle(lipgloss.NormalBorder()).
			Margin(0).Padding(0, 1, 1, 2)
	pagerStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			Margin(0).Padding(0)
	helpStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top).
			Foreground(lipgloss.Color("#CCC")).
			Italic(true).
			Margin(0).Padding(0)
)

type pagerKeymaps struct {
	top    key.Binding
	bottom key.Binding
	up     key.Binding
	down   key.Binding
	exit   key.Binding
}

var _ help.KeyMap = pagerKeymaps{}

func (k pagerKeymaps) ShortHelp() []key.Binding {
	return []key.Binding{k.exit, k.up, k.down}
}

func (k pagerKeymaps) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.top, k.bottom}, {k.up, k.down}, {k.exit}}
}

type listKeymaps struct {
	top    key.Binding
	bottom key.Binding
	up     key.Binding
	down   key.Binding
	jump   key.Binding
	open   key.Binding
	exit   key.Binding
}

var _ help.KeyMap = listKeymaps{}

func (k listKeymaps) ShortHelp() []key.Binding {
	return []key.Binding{k.exit, k.open}
}

func (k listKeymaps) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.exit, k.open}, {k.up, k.down, k.jump}}
}

var (
	listKeymap = listKeymaps{
		top: key.NewBinding(
			key.WithKeys("gg"),
			key.WithHelp("gg", "top"),
		),
		bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		jump: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("1-9", "jump"),
		),
		exit: key.NewBinding(
			key.WithKeys("esc", "-", "ctrl-c", "q"),
			key.WithHelp("q/esc", "exit"),
		),
		open: key.NewBinding(
			key.WithKeys("enter", "o", "p"),
			key.WithHelp("enter", "open"),
		),
	}

	pagerKeymap = pagerKeymaps{
		top: key.NewBinding(
			key.WithKeys("gg"),
			key.WithHelp("gg", "top"),
		),
		bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		exit: key.NewBinding(
			key.WithKeys("esc", "-", "ctrl-c", "q"),
			key.WithHelp("q/esc", "exit"),
		),
	}
)
