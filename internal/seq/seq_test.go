package seq

import "testing"

func TestContainsSequence(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		seq   []string
		ok    bool
	}{
		{
			name:  "exact match",
			lines: []string{"a", "b", "c"},
			seq:   []string{"a", "b", "c"},
			ok:    true,
		},
		{
			name:  "subsequence with extras",
			lines: []string{"x", "a", "y", "b", "z", "c"},
			seq:   []string{"a", "b", "c"},
			ok:    true,
		},
		{
			name:  "missing element",
			lines: []string{"a", "c"},
			seq:   []string{"a", "b", "c"},
			ok:    false,
		},
		{
			name:  "out of order",
			lines: []string{"b", "a"},
			seq:   []string{"a", "b"},
			ok:    false,
		},
		{
			name:  "element appears outside matched position",
			lines: []string{"a", "b", "a"},
			seq:   []string{"a", "b"},
			ok:    false,
		},
		{
			name:  "empty seq",
			lines: []string{"a", "b"},
			seq:   []string{},
			ok:    true,
		},
		{
			name:  "empty lines",
			lines: []string{},
			seq:   []string{"a"},
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := ContainsSequence(tt.lines, tt.seq...)
			if ok != tt.ok {
				t.Errorf("ContainsSequence = %v, want %v (reason: %s)", ok, tt.ok, reason)
			}
		})
	}
}
