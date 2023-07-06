package runner

import "io"

func NewTUI() UI     { return newTUI() }
func NewPrinter() UI { return newPrinter() }

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
