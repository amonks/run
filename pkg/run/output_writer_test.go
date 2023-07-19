package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputWriter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inputs  []string
		outputs []string
	}{
		{
			name:    "line buffering",
			inputs:  []string{"h", "e", "l", "l", "o", "\n", "w", "o", "r", "l", "d", "\n"},
			outputs: []string{"hello\n", "world\n"},
		},
		{
			name:    "line buffering without final newline (never prints)",
			inputs:  []string{"hello world"},
			outputs: nil,
		},
		{
			name:    "json",
			inputs:  []string{`{"apple": "banana", "tree": "bush"}` + "\n"},
			outputs: []string{"{\n  \"apple\": \"banana\",\n  \"tree\": \"bush\"\n}\n"},
		},
		{
			name:    "multipart json object",
			inputs:  []string{`{"apple":`, `"banana",`, `"tree":`, `"bush"}`, "\n"},
			outputs: []string{"{\n  \"apple\": \"banana\",\n  \"tree\": \"bush\"\n}\n"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var tw testWriter
			w := newOutputWriter(&tw)
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
