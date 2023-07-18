package run

import (
	_ "embed"
	"strings"
)

//go:generate go run github.com/amonks/run/cmd/licenses CREDITS.txt
//go:embed CREDITS.txt
var Credits string

//go:embed LICENSE.md
var License string

//go:embed CONTRIBUTORS.md
var contributors string
var Contributors string

func init() {
	var b strings.Builder
	for _, line := range strings.Split(contributors, "\n") {
		if strings.HasPrefix(line, "- ") {
			b.WriteString(line + "\n")
		}
	}
	Contributors = b.String()
}
