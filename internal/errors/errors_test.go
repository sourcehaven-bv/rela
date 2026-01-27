package errors

import (
	"errors"
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
