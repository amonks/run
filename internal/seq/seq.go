// Package seq provides helpers for asserting that a sequence of strings
// appears in order within a larger list of lines.
package seq

import (
	"fmt"
	"strings"
	"testing"
)

// ContainsSequence reports whether lines contains seq as an ordered
// subsequence, and that no element of seq appears anywhere in lines outside
// of the matched positions. It returns true if the subsequence is found
// cleanly, or false with a descriptive reason otherwise.
func ContainsSequence(lines []string, seq ...string) (bool, string) {
	if len(seq) == 0 {
		return true, ""
	}

	// Find the subsequence positions.
	positions := make([]int, 0, len(seq))
	si := 0
	for li, line := range lines {
		if si < len(seq) && line == seq[si] {
			positions = append(positions, li)
			si++
		}
	}
	if si < len(seq) {
		return false, fmt.Sprintf("subsequence not found: missing %q at position %d", seq[si], si)
	}

	// Check that no seq element appears outside its matched position.
	matched := make(map[int]struct{}, len(positions))
	for _, p := range positions {
		matched[p] = struct{}{}
	}
	seqSet := make(map[string]struct{}, len(seq))
	for _, s := range seq {
		seqSet[s] = struct{}{}
	}
	for li, line := range lines {
		if _, isMatched := matched[li]; isMatched {
			continue
		}
		if _, inSeq := seqSet[line]; inSeq {
			return false, fmt.Sprintf("element %q appears at line %d, outside the expected subsequence", line, li)
		}
	}

	return true, ""
}

// AssertContainsSequence is a test helper that asserts lines contains seq as
// a clean ordered subsequence.
func AssertContainsSequence(t *testing.T, lines []string, seq ...string) {
	t.Helper()
	ok, reason := ContainsSequence(lines, seq...)
	if !ok {
		t.Errorf("sequence assertion failed: %s\nlines: %v\nseq:   %v", reason, lines, seq)
	}
}

// AssertStringContainsSequence splits s on newlines and asserts the
// subsequence.
func AssertStringContainsSequence(t *testing.T, s string, seq ...string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	AssertContainsSequence(t, lines, seq...)
}
