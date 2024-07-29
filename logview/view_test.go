package logview

import "testing"

var (
	onetwothree = "111\n222\n333"
)

func TestClippers(t *testing.T) {
	for _, tc := range []struct {
		title  string
		f      func(string, int) string
		input  string
		output string
		n      int
	}{
		{"last n", lastNLines, onetwothree, "222\n333", 2},
		{"first n", firstNLines, onetwothree, "111\n222", 2},
		{"first n", firstNLines, onetwothree, "111\n222", 2},
		{"first n (too few)", firstNLines, onetwothree, "111\n222\n333", 4},
		{"last n (too few)", lastNLines, onetwothree, "111\n222\n333", 4},
	} {
		t.Run(tc.title, func(t *testing.T) {
			if got := tc.f(tc.input, tc.n); got != tc.output {
				t.Errorf("bad output:\n%s\n---\nexpected:\n%s", got, tc.output)
			}
		})
	}
}
