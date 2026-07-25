package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputWriter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pretty  bool
		inputs  []string
		outputs []string
	}{
		{
			name:    "line buffering",
			pretty:  true,
			inputs:  []string{"h", "e", "l", "l", "o", "\n", "w", "o", "r", "l", "d", "\n"},
			outputs: []string{"hello\n", "world\n"},
		},
		{
			name:    "line buffering without final newline (never prints)",
			pretty:  true,
			inputs:  []string{"hello world"},
			outputs: nil,
		},
		{
			name:    "json",
			pretty:  true,
			inputs:  []string{`{"apple": "banana", "tree": "bush"}` + "\n"},
			outputs: []string{"{\n  \"apple\": \"banana\",\n  \"tree\": \"bush\"\n}\n"},
		},
		{
			name:    "multipart json object",
			pretty:  true,
			inputs:  []string{`{"apple":`, `"banana",`, `"tree":`, `"bush"}`, "\n"},
			outputs: []string{"{\n  \"apple\": \"banana\",\n  \"tree\": \"bush\"\n}\n"},
		},
		{
			// When pretty is disabled (non-interactive output, e.g. under a
			// supervisor shipping to syslog), JSON lines pass through verbatim
			// so one log event stays one line.
			name:    "pretty disabled leaves json compact",
			pretty:  false,
			inputs:  []string{`{"apple": "banana", "tree": "bush"}` + "\n"},
			outputs: []string{`{"apple": "banana", "tree": "bush"}` + "\n"},
		},
		{
			name:    "pretty disabled still line buffers",
			pretty:  false,
			inputs:  []string{"h", "e", "l", "l", "o", "\n"},
			outputs: []string{"hello\n"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var tw testWriter
			w := newOutputWriter(&tw, tc.pretty)
			for _, input := range tc.inputs {
				w.Write([]byte(input))
			}
			assert.Equal(t, tc.outputs, tw.writes)
		})
	}
}

type testWriter struct {
	writes []string
}

func (w *testWriter) Write(bs []byte) (int, error) {
	w.writes = append(w.writes, string(bs))
	return len(bs), nil
}
