package entitymanager

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// mapUniquePropertyConflict must translate a store-level derived-unique-index
// violation into the SAME ValidationError the pre-write scan produces, so a
// client cannot tell which path caught the duplicate (TKT-3Q0GP1).
func TestMapUniquePropertyConflict(t *testing.T) {
	t.Run("named property maps to ValidationErrorUnique", func(t *testing.T) {
		ok, err := mapUniquePropertyConflict(store.UniquePropertyError{Property: "email"})
		if !ok {
			t.Fatal("expected mapping to occur")
		}
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want *ValidationError, got %T: %v", err, err)
		}
		if len(ve.Errors) != 1 {
			t.Fatalf("want 1 inner error, got %d", len(ve.Errors))
		}
		inner := ve.Errors[0]
		if inner.Type != metamodel.ValidationErrorUnique {
			t.Errorf("type = %q, want %q", inner.Type, metamodel.ValidationErrorUnique)
		}
		if inner.Property != "email" {
			t.Errorf("property = %q, want email", inner.Property)
		}
	})

	t.Run("unattributed violation still maps, without a property name", func(t *testing.T) {
		ok, err := mapUniquePropertyConflict(store.UniquePropertyError{})
		if !ok {
			t.Fatal("expected mapping to occur")
		}
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want *ValidationError, got %T", err)
		}
		if ve.Errors[0].Property != "" {
			t.Errorf("property = %q, want empty", ve.Errors[0].Property)
		}
	})

	t.Run("non-unique error passes through unchanged", func(t *testing.T) {
		orig := errors.New("some other write error")
		if ok, got := mapUniquePropertyConflict(orig); ok || !errors.Is(got, orig) {
			t.Errorf("expected pass-through, got %v (ok=%v)", got, ok)
		}
		// ErrConflict (ID collision) must NOT be turned into a validation error.
		if ok, _ := mapUniquePropertyConflict(store.ErrConflict); ok {
			t.Error("ErrConflict should pass through, not map")
		}
	})
}
