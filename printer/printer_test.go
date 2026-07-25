package printer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// When color is disabled (non-interactive output — piped to a file or shipped
// to syslog by a supervisor) the printer must not emit any ANSI escape codes.
// The gutter task ID must still be present so lines remain attributable.
func TestPrinterNoColorHasNoANSI(t *testing.T) {
	var sb strings.Builder
	p := New(len("monks_air"), &sb, false)
	p.Writer("monks_air").Write([]byte(`{"level":"INFO","msg":"hello"}` + "\n"))

	out := sb.String()
	assert.NotContains(t, out, "\x1b", "no-color output must contain no ANSI escape sequences")
	assert.Contains(t, out, "monks_air", "gutter task ID should still be printed")
	assert.Contains(t, out, `{"level":"INFO","msg":"hello"}`, "content should pass through")
}

// With color enabled the gutter and content are still both present; we do not
// assert on escape codes here because lipgloss strips them when the test's own
// stdout is not a color-capable terminal.
func TestPrinterColorPrintsContent(t *testing.T) {
	var sb strings.Builder
	p := New(len("monks_air"), &sb, true)
	p.Writer("monks_air").Write([]byte("hello\n"))

	out := sb.String()
	assert.Contains(t, out, "monks_air")
	assert.Contains(t, out, "hello")
}
