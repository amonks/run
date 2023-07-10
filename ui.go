package runner

import "io"

// NewTUI produces an interactive terminal UI for displaying mulitplexed
// streams. The UI shows a list of the streams, and allows keyboard and mouse
// navigation for selecting a particular stream to inspect.
//
// The UI can be passed into [Run.Start] to display a run's execution.
//
// The UI is safe to access concurrently from multiple goroutines.
func NewTUI() UI     { return newTUI() }

// NewPrinter produces a non-interactive UI for displaying interleaved
// multiplexed streams. The UI prints interleaved output from all of the
// streams to its Stdout. The output is suitable for piping to a file.
//
// The UI can be passed into [Run.Start] to display a run's execution.
//
// The UI is safe to access concurrently from multiple goroutines.
func NewPrinter() UI { return newPrinter() }

// A UI is essentially a multiplexed [io.Writer] that can be started and
// stopped. Since UIs implement [MultiWriter], they can be passed into
// [Run.Start] to display run execution.
//
// The functions [NewTUI] and [NewPrinter] produce implementors of UI.
type UI interface {
	Start(stdin io.Reader, stdout io.Writer, ids []string) error
	Wait() <-chan error
	Stop() error
	Writer(id string) io.Writer
}

// UIs implement MultiWriter
func init() {
	var ui UI = nil
	var _ MultiWriter = ui
}
