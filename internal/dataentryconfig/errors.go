package dataentryconfig

import "strings"

// ConfigValidationError collects multiple validation issues found in data-entry config.
type ConfigValidationError struct {
	Errors []string
}

func (e *ConfigValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	return "data-entry config validation errors:\n  - " + strings.Join(e.Errors, "\n  - ")
}
