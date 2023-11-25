package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/amonks/run/internal/color"
	"github.com/amonks/run/internal/logview"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

func newTUI(run *Run) UI {
	zone.NewGlobal()
	return &tui{mu: newMutex("tui"), run: run}
}

type tui struct {
	mu *mutex

	run *Run

	// nil until started
	p *tea.Program

	interleaved UI
}

// *tui implements MultiWriter
var _ MultiWriter = &tui{}

func (a *tui) Writer(id string) io.Writer {
	defer a.mu.Lock("Writer").Unlock()

	if a.p == nil {
		panic("getting writer from unstarted tui")
	}

	return tuiWriter{
		mu:                newMutex("writer:" + id),
		id:                id,
		interleavedWriter: a.interleaved,
		send:              a.p.Send,
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

func (a *tui) Start(ctx context.Context, ready chan<- struct{}, stdin io.Reader, stdout io.Writer) error {
	program := tea.NewProgram(
		&tuiModel{
			tui:    a,
			ids:    append([]string{"interleaved"}, a.run.IDs()...),
			onInit: func() { ready <- struct{}{} },
		},
		tea.WithAltScreen(),
		tea.WithContext(ctx),
		tea.WithMouseCellMotion())
	a.p = program

	interleavedWriter := a.Writer("interleaved")
	p := NewPrinter(a.run)
	go p.Start(ctx, nil, nil, interleavedWriter)
	a.interleaved = p

	exit := make(chan error)

	go func() {
		// run the bubbletea Program
		if _, err := program.Run(); err != nil && err != tea.ErrProgramKilled {
			exit <- err
			return
		}

		// When it exits, notify Waiters that the UI is done.
		exit <- nil
	}()

	err := <-exit

	return err
}

type tuiModel struct {
	tui *tui

	onInit func()

	ids       []string
	listWidth int

	width  int
	height int

	lastkey string
	didInit bool
	gotSize bool

	isPaging bool

	activeTask string

	tasks map[string]*logview.Model

	shortSpinner spinner.Model
	longSpinner  spinner.Model
	help         help.Model
	list         list.Model
}

func (m *tuiModel) Init() tea.Cmd {
	fmt.Fprintln(logfile, "init")

	m.tasks = map[string]*logview.Model{}
	for _, id := range m.ids {
		lv := logview.New()
		newLogview, _ := lv.Update(lv.SetWrapModeMsg(true))
		m.tasks[id] = newLogview.(*logview.Model)
	}

	m.shortSpinner = spinner.New()
	m.shortSpinner.Spinner = spinner.Jump
	m.longSpinner = spinner.New()
	m.longSpinner.Spinner = spinner.Hamburger

	longestKey := 0
	items := make([]list.Item, len(m.ids))
	for i, id := range m.ids {
		if len(id) > longestKey {
			longestKey = len(id)
		}
		items[i] = listItem(id)
	}
	m.listWidth = longestKey + 10
	m.list = list.New(items, itemDelegate{m}, 0, 0)
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
	m.list.SetShowHelp(false)
	m.list.SetShowTitle(false)

	m.help.ShowAll = true
	m.help.ShortSeparator = "   "
	m.help.FullSeparator = "     "

	m.didInit = true

	cmd := func() tea.Msg { return initializedMsg{} }
	return tea.Batch(cmd, m.shortSpinner.Tick, m.longSpinner.Tick)
}

type initializedMsg struct{}

type writeMsg struct {
	key     string
	content string
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {

	case initializedMsg:
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

		lv := m.tasks[m.activeTask]

		switch true {
		case lv.Focus == logview.FocusSearchBar:
			newLogview, cmd := lv.Update(msg)
			m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)

		case msg.String() == "g":
			if m.lastkey == "g" {
				if m.isPaging {
					newLogview, cmd := lv.Update(lv.ScrollToMsg(0))
					m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)
				} else {
					m.list.Select(0)
				}
			}
		case key.Matches(msg, keymap.bottom):
			if m.isPaging {
				newLogview, cmd := lv.Update(lv.ScrollToMsg(-1))
				m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)
			} else {
				m.list.Select(len(m.tasks) - 1)
			}
		case key.Matches(msg, keymap.up):
			if m.isPaging {
				newLogview, cmd := lv.Update(lv.ScrollByMsg(-1))
				m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)
			} else {
				m.list.CursorUp()
			}
		case key.Matches(msg, keymap.down):
			if m.isPaging {
				newLogview, cmd := lv.Update(lv.ScrollByMsg(1))
				m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)
			} else {
				m.list.CursorDown()
			}

		case key.Matches(msg, keymap.exit):
			if m.isPaging {
				m.isPaging = false
			} else {
				return m, tea.Quit
			}

		case key.Matches(msg, keymap.toggleWrap):
			newLogview, cmd := lv.Update(lv.ToggleWrapModeMsg())
			m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)

		case key.Matches(msg, keymap.search):
			newLogview, cmd := lv.Update(lv.SetFocusMsg(logview.FocusSearchBar))
			m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)

		case key.Matches(msg, keymap.nextResult):
			newLogview, cmd := lv.Update(lv.MoveResultIndexMsg(1))
			m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)

		case key.Matches(msg, keymap.prevResult):
			newLogview, cmd := lv.Update(lv.MoveResultIndexMsg(-1))
			m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)

		case key.Matches(msg, keymap.jump):
			n, err := strconv.Atoi(msg.String())
			if err != nil {
				panic(err)
			}
			i := n
			if i < len(m.ids) {
				m.list.Select(i)
				// XXX: shouldn't this set m.activeTask?
			}

		case key.Matches(msg, keymap.focus):
			m.isPaging = true

		case key.Matches(msg, keymap.write):
			m.writeFile()

		case key.Matches(msg, keymap.restart):
			m.tui.run.Invalidate(string(m.list.SelectedItem().(listItem)))
		}

		m.lastkey = msg.String()

	case writeMsg:
		lv := m.tasks[msg.key]
		newLogview, cmd := lv.Update(lv.WriteMsg(msg.content))
		m.tasks[msg.key], cmds = newLogview.(*logview.Model), append(cmds, cmd)

	case tea.WindowSizeMsg:
		// model
		m.width = msg.Width
		m.height = msg.Height

		// help
		helpStyle = helpStyle.
			UnsetMaxWidth().MaxWidth(m.width).
			UnsetMaxHeight().MaxHeight(helpHeight)
		helpStyle = helpStyle.
			UnsetWidth().Width(helpStyle.GetMaxWidth() - helpStyle.GetHorizontalFrameSize()).
			UnsetHeight().Height(helpStyle.GetMaxHeight() - helpStyle.GetVerticalFrameSize())

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

		// logviews
		for k, l := range m.tasks {
			newLogview, cmd := l.Update(l.SetDimensionsMsg(m.width-m.listWidth, m.height-helpHeight))
			m.tasks[k], cmds = newLogview.(*logview.Model), append(cmds, cmd)
		}

		// done
		m.gotSize = true

	case spinner.TickMsg:
		var cmd1 tea.Cmd
		var cmd2 tea.Cmd
		m.shortSpinner, cmd1 = m.shortSpinner.Update(msg)
		m.longSpinner, cmd2 = m.longSpinner.Update(msg)
		cmds = append(cmds, cmd1, cmd2)

	default:
		cmds = append(cmds, m.passthrough(msg)...)
	}

	if item := m.list.SelectedItem(); item != nil {
		selectedItem := string(item.(listItem))
		if selectedItem != m.activeTask {
			m.activeTask = selectedItem
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) passthrough(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	activeLogview := m.tasks[m.activeTask]
	newLogview, cmd := activeLogview.Update(msg)
	m.tasks[m.activeTask], cmds = newLogview.(*logview.Model), append(cmds, cmd)

	newList, cmd := m.list.Update(msg)
	m.list, cmds = newList, append(cmds, cmd)

	return cmds
}

func (m *tuiModel) writeFile() {
	filename := m.activeTask + ".log"
	filename = strings.Replace(filename, string(os.PathSeparator), "-", -1)
	content := stripANSIEscapeCodes(m.tasks[m.activeTask].String())
	os.WriteFile(filename, []byte(content), 0644)

	logMsg := fmt.Sprintf("wrote log to '%s'", filename)
	go m.tui.p.Send(writeMsg{key: m.activeTask, content: fmt.Sprintln(logStyle.Render(logMsg))})
}

func (m *tuiModel) View() string {
	if !m.didInit || !m.gotSize {
		return "......."
	}

	activeLogview := m.tasks[m.activeTask]
	return zone.Scan(lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			listStyle.Render(m.list.View()),
			activeLogview.View(),
		),
		helpStyle.Render(m.help.View(keymap)),
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

	spinner := " "
	status := d.m.tui.run.TaskStatus(string(i))
	switch status {
	case TaskStatusNotStarted:
		spinner = " "
	case TaskStatusRunning:
		if d.m.tui.run.tasks[string(i)].Metadata().Type == "long" {
			spinner = d.m.longSpinner.View()
		} else {
			spinner = d.m.shortSpinner.View()
		}
	case TaskStatusRestarting:
		spinner = d.m.shortSpinner.View()
	case TaskStatusFailed:
		spinner = "×"
	case TaskStatusDone:
		spinner = "✓"
	}

	var str string
	var marker string
	style := listItemStyle.Copy().Foreground(lipgloss.Color(color.Hash(id)))
	if index == m.Index() {
		marker = ">"
	} else {
		marker = " "
	}
	str = fmt.Sprintf("%s %s %d. %s", spinner, marker, index, style.Render(id))
	fmt.Fprint(w, zone.Mark(id, str))
}

type keymaps struct {
	top    key.Binding
	bottom key.Binding
	up     key.Binding
	down   key.Binding
	exit   key.Binding

	toggleWrap key.Binding
	search     key.Binding
	nextResult key.Binding
	prevResult key.Binding

	jump    key.Binding
	focus   key.Binding
	write   key.Binding
	restart key.Binding
}

var _ help.KeyMap = keymaps{}

func (k keymaps) ShortHelp() []key.Binding {
	return []key.Binding{k.exit, k.up, k.down}
}

func (k keymaps) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.top, k.bottom}, {k.up, k.down}, {k.exit}}
}

var (
	keymap = keymaps{
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

		toggleWrap: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "wrap"),
		),
		search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		nextResult: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("/", "next result"),
		),
		prevResult: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("/", "prev result"),
		),

		jump: key.NewBinding(
			key.WithKeys("0", "1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("0-9", "jump"),
		),
		focus: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "focus on this log"),
		),
		write: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "write log to file"),
		),
		restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart task"),
		),
	}
)
