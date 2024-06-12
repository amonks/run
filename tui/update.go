package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/amonks/run/internal/ansi"
	"github.com/amonks/run/internal/help"
	"github.com/amonks/run/pkg/logview"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case msgInitialized:
		m.onInit()
		return m, nil

	case tea.MouseMsg:
		if m.focus == focusHelp {
			return m.passthroughToHelp(msg)
		}

		if msg.Button == tea.MouseButtonLeft {
			status := m.tui.runner.Status()
			for i, id := range status.AllTasks {
				if zone.Get(id).InBounds(msg) {
					m.selectedTaskIDIndex = i
					m.focus = focusMenu
					return m, nil
				}
			}
			return m, nil
		}

		if zone.Get(uiZoneLogs).InBounds(msg) {
			return m.passthroughToLogview(msg)
		}

		return m, nil

	case tea.KeyMsg:
		if !m.didInit || !m.gotSize {
			return m, nil
		}

		// handle ctrl+c -- regarldess of mode, this should always quit.
		if msg.String() == "ctrl+c" {
			return m.handleQuitAttempt(msg.String())
		}
		if m.quitKey == msg.String() {
			return m, tea.Quit
		}
		m.quitKey = ""

		// If we're focused on the full-screen help viewer, the only possible
		// actions are manipulating or exiting it..
		if m.focus == focusHelp {
			switch msg.String() {
			case "h", "?", "esc", "q":
				m.focus = focusMenu
				return m, nil
			default:
				newHelp, cmd := m.help.Update(msg)
				m.help = newHelp
				return m, cmd
			}
		}

		// We're looking at a task.
		lv := m.tasks[m.activeTaskID()]

		// Always update m.lastKey, as a convenience. The rest of this
		// function should read from lastKey instead.
		lastkey := m.lastkey
		m.lastkey = msg.String()

		// If we're in search, we want the text input to completely
		// hijack the keyboard.
		if m.focus == focusSearch {
			newLogview, cmd := lv.Update(msg)
			m.tasks[m.activeTaskID()] = newLogview.(*logview.Model)

			// Delegate to the logview to tell us whether search
			// should still be focused.
			switch m.tasks[m.activeTaskID()].Focus() {
			case logview.FocusLogPane:
				m.focus = focusLogs
			}
			return m, cmd
		}

		// Handle keyboard shortcuts.
		switch msg.String() {
		case "ctrl+c":
			return m.handleQuitAttempt("ctrl+c")

		case "esc", "q":
			// This -could- be a quit attempt, or we could just be
			// deselecting a log.
			switch m.focus {
			case focusLogs:
				m.focus = focusMenu
			case focusMenu:
				return m.handleQuitAttempt(msg.String())
			}
			return m, nil

		case "?":
			m.focus = focusHelp
			return m, nil

		case "/":
			m.focus = focusSearch
			lv.SetFocus(logview.FocusSearchBar)
			return m, nil

		case "n":
			lv.MoveResultIndex(1)
			return m, nil

		case "N":
			lv.MoveResultIndex(-1)
			return m, nil

		case "tab":
			// toggle between logs and the menu.
			switch m.focus {
			case focusLogs:
				m.focus = focusMenu
			case focusMenu:
				m.focus = focusLogs
			}
			return m, nil

		case "enter", "l":
			switch m.focus {
			case focusMenu:
				m.focus = focusLogs
			}
			return m, nil

		case "h":
			switch m.focus {
			case focusLogs:
				m.focus = focusMenu
			}
			return m, nil

		case "w":
			lv.ToggleWrapMode()
			return m, nil

		case "s":
			m.writeFile()
			return m, nil

		case "g":
			if lastkey != "g" {
				return m, nil
			}
			switch m.focus {
			case focusLogs:
				lv.ScrollTo(0)
			case focusMenu:
				m.selectedTaskIDIndex = 0
			}
			return m, nil
		case "G":
			switch m.focus {
			case focusLogs:
				lv.ScrollTo(-1)
			case focusMenu:
				styles := m.styles(m.width, m.height, m.focus)
				m.selectedTaskIDIndex = len(styles.visibleMenuItems) - 1
			}
			return m, nil

		case "home":
			lv.ScrollTo(0)
			return m, nil
		case "end":
			lv.ScrollTo(-1)
			return m, nil

		case "k", "up":
			switch m.focus {
			case focusLogs:
				lv.ScrollBy(-1)
			case focusMenu:
				styles := m.styles(m.width, m.height, m.focus)
				m.selectedTaskIDIndex -= 1
				if m.selectedTaskIDIndex < 0 {
					m.selectedTaskIDIndex = len(styles.visibleMenuItems) - 1
				}
			}
			return m, nil
		case "j", "down":
			switch m.focus {
			case focusLogs:
				lv.ScrollBy(1)
			case focusMenu:
				styles := m.styles(m.width, m.height, m.focus)
				m.selectedTaskIDIndex += 1
				if m.selectedTaskIDIndex >= len(styles.visibleMenuItems) {
					m.selectedTaskIDIndex = 0
				}
			}
			return m, nil

		case "pgup":
			styles := m.styles(m.width, m.height, m.focus)
			pageDistance := max(0, styles.logHeight-2)
			lv.ScrollBy(-pageDistance)
			return m, nil
		case "pgdown":
			styles := m.styles(m.width, m.height, m.focus)
			pageDistance := max(0, styles.logHeight-2)
			lv.ScrollBy(pageDistance)
			return m, nil

		case "ctrl+u":
			styles := m.styles(m.width, m.height, m.focus)
			lv.ScrollBy(-styles.logHeight / 2)
			return m, nil
		case "ctrl+d":
			styles := m.styles(m.width, m.height, m.focus)
			lv.ScrollBy(styles.logHeight / 2)
			return m, nil

		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			status := m.tui.runner.Status()
			n, err := strconv.Atoi(msg.String())
			if err != nil {
				panic(err)
			}
			i := n
			if i < len(status.AllTasks) {
				m.selectedTaskIDIndex = i
				// XXX: shouldn't this set m.activeTask?
			}
			return m, nil

		case "r":
			m.tui.runner.Invalidate(m.activeTaskID())
			return m, nil

		default:
			return m, nil
		}

	case msgWrite:
		lv := m.tasks[msg.key]
		if lv == nil {
			panic("logview " + msg.key + " not initialized")
		}
		lv.Write(msg.content)
		return m, nil

	case tea.WindowSizeMsg:
		m.help.Width, m.help.Height = msg.Width, msg.Height
		m.help.SetContent(helpMenu.Render(help.Colored, msg.Width, msg.Height))
		m.width, m.height = msg.Width, msg.Height
		m.gotSize = true
		return m, nil

	case spinner.TickMsg:
		var cmd1 tea.Cmd
		var cmd2 tea.Cmd
		m.shortSpinner, cmd1 = m.shortSpinner.Update(msg)
		m.longSpinner, cmd2 = m.longSpinner.Update(msg)
		return m, tea.Batch(cmd1, cmd2)

	default:
		return m.passthroughToLogview(msg)
	}
}

func (m *Model) passthroughToLogview(msg tea.Msg) (*Model, tea.Cmd) {
	activeLogview := m.tasks[m.activeTaskID()]
	newLogview, cmd := activeLogview.Update(msg)
	m.tasks[m.activeTaskID()] = newLogview.(*logview.Model)
	return m, cmd
}

func (m *Model) passthroughToHelp(msg tea.Msg) (*Model, tea.Cmd) {
	newHelp, cmd := m.help.Update(msg)
	m.help = newHelp
	return m, cmd
}

func (m *Model) handleQuitAttempt(key string) (*Model, tea.Cmd) {
	if m.quitKey == key {
		return m, tea.Quit
	}
	m.quitKey = key
	return m, nil
}

func (m *Model) writeFile() {
	filename := m.activeTaskID() + ".log"
	filename = strings.Replace(filename, string(os.PathSeparator), "-", -1)
	content := ansi.Strip(m.tasks[m.activeTaskID()].String())
	os.WriteFile(filename, []byte(content), 0644)

	logMsg := fmt.Sprintf("wrote log to '%s'", filename)
	go m.tui.p.Send(msgWrite{key: m.activeTaskID(), content: fmt.Sprintln(logStyle.Render(logMsg))})
}
