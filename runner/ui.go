package runner

import (
	"context"
	"io"
)

// A UI is essentially a multiplexed [io.Writer] that can be started and
// stopped. Since UIs implement [MultiWriter], they can be passed into
// [New] to display run execution.
//
// The package [printer] produces implementors of UI.
type UI interface {
	Start(ctx context.Context, ready chan<- struct{}, stdin io.Reader, stdout io.Writer) error
	Writer(id string) io.Writer
}

// UIs implement MultiWriter
func init() {
	var ui UI = nil
	var _ MultiWriter = ui
}
