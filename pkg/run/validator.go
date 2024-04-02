package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type validator struct{ cwd string }

func newValidator() validator {
	return validator{}
}

func newValidatorWithCWD(cwd string) (validator, error) {
	if abs, err := filepath.Abs(cwd); err != nil {
		return validator{}, err
	} else {
		return validator{abs}, nil
	}
}

func (v validator) validate(ts Tasks) error {
	var problems []string

	ids := map[string]struct{}{}
	for _, id := range ts.IDs() {
		t := ts.Get(id)
		if id != t.Metadata().ID {
			problems = append(problems, fmt.Sprintf("- task '%s' has mismatched key '%s'", t.Metadata().ID, id))
		}
		ids[t.Metadata().ID] = struct{}{}
	}
	for _, id := range ts.IDs() {
		t := ts.Get(id)
		for _, err := range v.validateTask(ts, t) {
			problems = append(problems, "- "+err.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(append([]string{"invalid taskfile"}, problems...), "\n"))
	}
	return nil
}

func (v validator) validateTask(ts Tasks, t Task) []error {
	var problems []error

	meta := t.Metadata()
	if meta.ID == "" {
		problems = append(problems, errors.New("Task has no ID."))
	}

	if meta.ID == "interleaved" || meta.ID == "run" {
		problems = append(problems, fmt.Errorf("'%s' is reserved and cannot be used as a task ID.", meta.ID))
	}

	for _, c := range meta.ID {
		if unicode.IsSpace(c) {
			problems = append(problems, fmt.Errorf("Task IDs cannot contain whitespace characters."))
		}
	}

	if meta.Type != "long" && meta.Type != "short" && meta.Type != "group" {
		problems = append(problems, fmt.Errorf("Task '%s' has invalid type '%s'; must be 'long' or 'short'.", meta.ID, meta.Type))
	}

	if meta.Type == "group" {
		problems = append(problems, fmt.Errorf("Task '%s' is a group, but group tasks are deprecated. Mark it as 'long' or 'short'. To preserve its current behavior, use 'long'.", meta.ID))
	}

	for _, id := range meta.Dependencies {
		if !ts.Has(id) {
			problems = append(problems, fmt.Errorf("Task '%s' lists dependency '%s', which is not the ID of a task.", meta.ID, id))
		}
	}

	for _, id := range meta.Triggers {
		if !ts.Has(id) {
			problems = append(problems, fmt.Errorf("Task '%s' lists trigger '%s', which is not the ID of a task.", meta.ID, id))
		}
		if ts.Get(id).Metadata().Type == "long" {
			problems = append(problems, fmt.Errorf("Task '%s' lists trigger '%s', which is long. Long tasks aren't expected to end, so using them as triggers is invalid.", meta.ID, id))
		}
	}

	for _, path := range meta.Watch {
		if strings.HasPrefix(path, string(os.PathSeparator)) {
			problems = append(problems, fmt.Errorf("Task '%s' wants to watch path '%s', which is absolute.", meta.ID, path))
		}
		if s, isScript := t.(*scriptTask); isScript {
			if abs, err := filepath.Abs(filepath.Join(s.dir, path)); err != nil {
				problems = append(problems, fmt.Errorf("Task '%s' had an error resolving path '%s': %s.", meta.ID, path, err))
			} else if !strings.HasPrefix(abs, v.cwd) {
				problems = append(problems, fmt.Errorf("Task '%s' wants to watch path '%s', which is outside of the working directory.", meta.ID, path))
			}
		}
	}

	return problems
}
