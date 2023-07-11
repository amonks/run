package run

import (
	_ "embed"
)

//go:generate go run github.com/amonks/run/cmd/licenses CREDITS.txt
//go:embed CREDITS.txt
var Credits string

//go:embed LICENSE.md
var License string
