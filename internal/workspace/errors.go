package workspace

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ValidationError wraps multiple validation errors from the metamodel.
type ValidationError struct {
	Errors []*metamodel.ValidationError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("validation errors:\n  %s", strings.Join(msgs, "\n  "))
}

func newValidationError(errs []*metamodel.ValidationError) *ValidationError {
	return &ValidationError{Errors: errs}
}

// IsValidationError returns true if the error is a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
