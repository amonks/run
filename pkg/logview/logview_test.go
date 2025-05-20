package logview

import (
	"testing"
)

var (
	oneextra = "aaaaa\nbbbbb\nccccc\nddddd\neeeee"
	alphabet = "abcdefghijklmnopqrstuvwxyz"

	searchwrapped   = "abca\nbcab\ncabc"
	searchunwrapped = "abcabcabcabc"
	searchResults   = string([]byte{
		27, 91, 51, 56, 59, 50, 59, 48, 59, 48, 59, 48, 59, 52, 56, 59, 50, 59, 50, 53, 53, 59, 50, 53, 53, 59, 48, 109, 97, 27, 91, 48, 109, 98, 99, 27, 91, 51, 56, 59, 50, 59, 48, 59, 48, 59, 48, 59, 52, 56, 59, 50, 59, 50, 53, 53, 59, 50, 53, 53, 59, 48, 109, 97, 27, 91, 48, 109, 10,
		98, 99, 27, 91, 51, 56, 59, 50, 59, 48, 59, 48, 59, 48, 59, 52, 56, 59, 50, 59, 50, 53, 53, 59, 50, 53, 53, 59, 48, 109, 97, 27, 91, 48, 109, 98, 10,
		99, 27, 91, 51, 56, 59, 50, 59, 48, 59, 48, 59, 48, 59, 52, 56, 59, 50, 59, 50, 53, 53, 59, 50, 53, 53, 59, 48, 109, 97, 27, 91, 48, 109, 98, 99,
	})
)

type mod = func(m *Model)

var (
	head = func(m *Model) { m.ScrollTo(0) }
	tail = func(m *Model) { m.ScrollTo(-1) }
	soft = func(m *Model) { m.SetWrapMode(false) }
	hard = func(m *Model) { m.SetWrapMode(true) }

	query = func(q string) mod { return func(m *Model) { m.SetQuery(q) } }
)

func TestLogview(t *testing.T) {
	// compat.SetColorProfile(termenv.TrueColor)
	for _, tc := range []struct {
		title  string
		input  string
		expect string
		mods   []mod
	}{
		{"hardwrap head oneextra", oneextra, "aaaa\nbbbb\ncccc", []mod{hard, head}},
		{"hardwrap tail oneextra", oneextra, "cccc\ndddd\neeee", []mod{hard, tail}},
		{"softwrap head oneextra", oneextra, "aaaa\na   \nbbbb", []mod{soft, head}},
		{"softwrap tail oneextra", oneextra, "d   \neeee\ne   ", []mod{soft, tail}},
		{"hardwrap head alphabet", alphabet, "abcd\n    \n    ", []mod{hard, head}},
		{"hardwrap tail alphabet", alphabet, "    \n    \nabcd", []mod{hard, tail}},
		{"softwrap head alphabet", alphabet, "abcd\nefgh\nijkl", []mod{soft, head}},
		{"softwrap tail alphabet", alphabet, "qrst\nuvwx\nyz  ", []mod{soft, tail}},
		{"search tail hardwrap", searchwrapped, searchResults, []mod{hard, query("a"), tail}},
		{"search tail softwrap", searchunwrapped, searchResults, []mod{soft, query("a"), tail}},
		{"search head hardwrap", searchwrapped, searchResults, []mod{hard, query("a"), head}},
		{"search head softwrap", searchunwrapped, searchResults, []mod{soft, query("a"), head}},
	} {
		t.Run(tc.title, func(t *testing.T) {
			testView(t, tc.input, tc.expect, tc.mods)
		})
	}
}

func testView(t *testing.T, input, expect string, mods []mod) {
	m := New(WithoutStatusbar)
	_ = m.Init()

	m.SetDimensions(4, 3)
	m.Write(input)
	for _, mod := range mods {
		mod(m)
	}

	if got := firstNLines(m.View(), 3); got != expect {
		t.Errorf("bad output:\n%s\n---- (%d bytes)\n%v\nexpected:\n%s\n---- (%d bytes)\n%v",
			got, len(got), []byte(got),
			expect, len(expect), []byte(expect))
	}
}
