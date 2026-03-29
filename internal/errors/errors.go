package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInvalidID       = errors.New("invalid entity ID")
	ErrInvalidType     = errors.New("invalid entity type")
	ErrInvalidRelation = errors.New("invalid relation")
	ErrNoProject       = errors.New("no project found (missing metamodel.yaml)")
	ErrValidation      = errors.New("validation error")
)

type EntityNotFoundError struct {
	ID string
}

func (e *EntityNotFoundError) Error() string {
	return fmt.Sprintf("entity not found: %s", e.ID)
}

func (e *EntityNotFoundError) Unwrap() error {
	return ErrNotFound
}

type EntityTypeNotFoundError struct {
	Type string
}

func (e *EntityTypeNotFoundError) Error() string {
	return fmt.Sprintf("unknown entity type: %s", e.Type)
}

func (e *EntityTypeNotFoundError) Unwrap() error {
	return ErrInvalidType
}

type RelationNotFoundError struct {
	Name string
}

func (e *RelationNotFoundError) Error() string {
	return fmt.Sprintf("unknown relation: %s", e.Name)
}

func (e *RelationNotFoundError) Unwrap() error {
	return ErrInvalidRelation
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

// ExitError is an error that indicates the program should exit with a specific code.
// This allows commands to signal exit codes without calling os.Exit directly,
// making them testable.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// NewExitError creates a new ExitError with the given exit code.
func NewExitError(code int) *ExitError {
	return &ExitError{Code: code}
}
