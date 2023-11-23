package logview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLogview(t *testing.T) {
	m := New()
	_ = m.Init()

	teaModel, _ := m.Update(tea.WindowSizeMsg{Width: 4, Height: 4})
	m = teaModel.(*Model)

	teaModel, _ = m.Update(writeMsg{m.id, "aaaaa\nbbbbb\nccccc\nddddd\neeeee"})
	m = teaModel.(*Model)

	// teaModel, _ = m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: false}))
	// m = teaModel.(*Model)

	if got := m.View(); got != "bbbb\ncccc\ndddd\neeee" {
		t.Errorf("bad output:\n%s\n---- (%d bytes)", got, len(got))
	}
}
