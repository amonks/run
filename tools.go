//go:build tools
// +build tools

package runner

import (
	_ "github.com/goreleaser/goreleaser"
	_ "golang.org/x/tools/cmd/stringer"
)
