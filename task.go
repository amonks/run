package runner

import "io"

type Task interface {
	Start(stdout io.Writer) error
	Wait() <-chan error
	Stop() error
	Metadata() TaskMetadata
}

type TaskMetadata struct {
	ID           string
	Type         string
	Dependencies []string
	Triggers     []string
	Watch        []string
	CWD          string
}
