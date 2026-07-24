package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestEntityNotFoundError(t *testing.T) {
	err := &EntityNotFoundError{ID: "test-123"}

	if err.Error() != "entity not found: test-123" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrNotFound) {
		t.Error("EntityNotFoundError should wrap ErrNotFound")
	}
}

func TestEntityTypeNotFoundError(t *testing.T) {
	err := &EntityTypeNotFoundError{Type: "unknown"}

	if err.Error() != "unknown entity type: unknown" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrInvalidType) {
		t.Error("EntityTypeNotFoundError should wrap ErrInvalidType")
	}
}

func TestRelationNotFoundError(t *testing.T) {
	err := &RelationNotFoundError{Name: "missing"}

	if err.Error() != "unknown relation: missing" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrInvalidRelation) {
		t.Error("RelationNotFoundError should wrap ErrInvalidRelation")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Field: "name", Message: "required"}

	if err.Error() != "validation error on name: required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrValidation) {
		t.Error("ValidationError should wrap ErrValidation")
	}
}

func TestExitError(t *testing.T) {
	err := NewExitError(42)

	if err.Code != 42 {
		t.Errorf("expected code 42, got %d", err.Code)
	}

	if err.Error() != "exit status 42" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestExitErrorZero(t *testing.T) {
	err := NewExitError(0)

	if err.Code != 0 {
		t.Errorf("expected code 0, got %d", err.Code)
	}

	if err.Error() != "exit status 0" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestWrapDiscoverError(t *testing.T) {
	t.Run("no project returns init hint", func(t *testing.T) {
		wrapped := fmt.Errorf("discover: %w", ErrNoProject)
		got := WrapDiscoverError(wrapped)
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		if msg := got.Error(); !strings.Contains(msg, "run 'rela init'") {
			t.Errorf("expected init hint, got: %q", msg)
		}
	})

	t.Run("other errors are surfaced verbatim", func(t *testing.T) {
		underlying := errors.New("load metamodel: yaml: line 3: mapping values are not allowed in this context")
		got := WrapDiscoverError(underlying)
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if strings.Contains(msg, "run 'rela init'") {
			t.Errorf("should not suggest init for load failures, got: %q", msg)
		}
		if !strings.Contains(msg, "yaml: line 3") {
			t.Errorf("expected underlying error to be surfaced, got: %q", msg)
		}
		if !errors.Is(got, underlying) {
			t.Errorf("expected underlying error preserved, got: %v", got)
		}
	})
}
