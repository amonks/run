//go:build tools
// +build tools

package run

import (
	_ "github.com/goreleaser/goreleaser"
	_ "golang.org/x/tools/cmd/stringer"
)
