package seq

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func AssertStringContainsSequence(t *testing.T, str string, seq ...string) {
	assert.NoError(t, StringContainsSequence(str, seq...))
}

func AssertContainsSequence(t *testing.T, lines []string, seq ...string) {
	assert.NoError(t, ContainsSequence(lines, seq...))
}

func StringContainsSequence(str string, seq ...string) error {
	return ContainsSequence(strings.Split(str, "\n"), seq...)
}

func ContainsSequence(lines []string, seq ...string) error {
	assertedLines := map[string]struct{}{}
	for _, l := range seq {
		assertedLines[l] = struct{}{}
	}

	lineIndex := 0
seqloop:
	for seqIndex, expect := range seq {
		for ; lineIndex < len(lines); lineIndex++ {
			line := lines[lineIndex]
			if line == expect {
				lineIndex++
				continue seqloop
			} else if _, isAsserted := assertedLines[line]; isAsserted {
				return fmt.Errorf(strings.Join([]string{
					"Found sequneced item outside of the sequence.",
					"Found: '%s'",
					"Looking for sequence item %d: '%s'",
					"",
					"Sequence:",
					"%s",
					"",
					"Actual:",
					"%s",
				}, "\n"),
					line,
					seqIndex+1, expect,
					strings.Join(seq, "\n"),
					strings.Join(lines, "\n"),
				)
			}
		}
		return fmt.Errorf(strings.Join([]string{
			"Not found in sequence.",
			"Item %d: '%s'",
			"",
			"Sequence:",
			"%s",
			"",
			"Actual:",
			"%s",
		}, "\n"),
			seqIndex+1, expect,
			strings.Join(seq, "\n"),
			strings.Join(lines, "\n"),
		)
	}

	// got through the seq; now check the rest of lines to make sure
	// expected lines didn't recur extra times.
	for ; lineIndex < len(lines); lineIndex++ {
		line := lines[lineIndex]
		if _, isAsserted := assertedLines[line]; isAsserted {
			return fmt.Errorf(strings.Join([]string{
				"Found outside of sequence.",
				"Found: '%s'",
				"Entire sequence already consumed.",
				"",
				"Sequence:",
				"%s",
				"",
				"Actual:",
				"%s",
			}, "\n"),
				line,
				strings.Join(seq, "\n"),
				strings.Join(lines, "\n"),
			)
		}
	}
	return nil
}
