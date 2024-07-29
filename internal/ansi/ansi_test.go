package ansi_test

import (
	"testing"

	"github.com/amonks/run/internal/ansi"
)

func TestStripANSIEscapeCodes(t *testing.T) {
	//lint:ignore ST1018 This is pasted literal output from Run. It's more
	//important that it be literal than that it exclude control characters.
	input := `[3;38;2;204;204;204mstarting[0m`
	expect := `starting`

	got := ansi.Strip(input)
	if got != expect {
		t.Errorf("stripAnsiEscapeCodes('%s') = '%s'; got '%s'", input, expect, got)
	}
}
