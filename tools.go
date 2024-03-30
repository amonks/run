//go:build tools
// +build tools

package run

import (
	_ "github.com/charmbracelet/vhs"
	_ "github.com/goreleaser/goreleaser"
	_ "golang.org/x/tools/cmd/stringer"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
