package tui

import "fmt"

func (msg msgInitialized) String() string {
	return fmt.Sprintf("[msgInitialized %#v]", msg)
}

func (msg msgWrite) String() string {
	return fmt.Sprintf("[msgWrite %#v]", msg)
}

