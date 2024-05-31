package seq_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/amonks/run/internal/seq"
	"github.com/stretchr/testify/assert"
)

func TestContainsSequence(t *testing.T) {
	for _, tc := range []struct {
		in, match []string
		out       bool
	}{
		{
			in:    []string{"a", "b", "c", "d"},
			match: []string{"b", "c"},
			out:   true,
		},
		{
			in:    []string{"a", "b", "c", "d"},
			match: []string{"a", "b"},
			out:   true,
		},
		{
			in:    []string{"a", "b", "c", "d"},
			match: []string{"-", "|"},
			out:   false,
		},
		{
			in:    []string{"a", "-", "a", "-", "a", "a"},
			match: []string{"-", "-"},
			out:   true,
		},
		{
			in:    []string{"a", "-", "a", "-", "a", "a"},
			match: []string{"a", "a"},
			out:   false,
		},
	} {
		modifier := " "
		if !tc.out {
			modifier = " not "
		}
		name := fmt.Sprintf("[%s]%sin [%s]", strings.Join(tc.match, ","), modifier, strings.Join(tc.in, ","))
		t.Run(name, func(t *testing.T) {
			err := seq.ContainsSequence(tc.in, tc.match...)
			if shouldHaveNoError := tc.out; shouldHaveNoError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
