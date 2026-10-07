package main

import (
	"errors"
	"fmt"
)

// invalidInputError marks an error caused by caller-supplied arguments, so
// handlers can answer 400 instead of blaming the upstream with a 500.
type invalidInputError struct{ msg string }

func (e *invalidInputError) Error() string { return e.msg }

// invalidInput builds a caller-facing validation error.
func invalidInput(format string, args ...any) error {
	return &invalidInputError{msg: fmt.Sprintf(format, args...)}
}

// isInvalidInput reports whether err (or anything it wraps) came from invalidInput.
func isInvalidInput(err error) bool {
	_, ok := errors.AsType[*invalidInputError](err)
	return ok
}
