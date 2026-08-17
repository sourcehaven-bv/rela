//go:build postgres

package pgstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// These are pure-unit tests for the derived-schema name/mapping logic; they
// need no database and so are not gated on RELA_TEST_DATABASE_URL.

func TestUniqueIndexName_DeterministicAndBounded(t *testing.T) {
	// Deterministic: same inputs -> same name across calls.
	if a, b := uniqueIndexName("persoon", "email"), uniqueIndexName("persoon", "email"); a != b {
		t.Fatalf("non-deterministic: %q != %q", a, b)
	}
	// Prefix and length: must be owned-prefixed and within Postgres's 63-byte
	// identifier limit.
	name := uniqueIndexName("persoon", "email")
	if !strings.HasPrefix(name, derivedUniquePrefix) {
		t.Errorf("name %q missing prefix %q", name, derivedUniquePrefix)
	}
	if len(name) > 63 {
		t.Errorf("name %q exceeds 63 bytes (%d)", name, len(name))
	}
	// A long type+property must still fit.
	long := uniqueIndexName(strings.Repeat("t", 200), strings.Repeat("p", 200))
	if len(long) > 63 {
		t.Errorf("long name exceeds 63 bytes (%d)", len(long))
	}
}

func TestUniqueIndexName_NoSeparatorCollision(t *testing.T) {
	// The NUL separator must make ("ab","c") and ("a","bc") distinct.
	if uniqueIndexName("ab", "c") == uniqueIndexName("a", "bc") {
		t.Error("index names collided across the (type,property) boundary")
	}
}

func TestMapUniqueViolation(t *testing.T) {
	pairs := []store.DerivedObjectSpec{
		{Kind: store.DerivedUnique, Type: "persoon", Property: "email"},
	}
	knownName := uniqueIndexName("persoon", "email")

	t.Run("non-owned constraint stays ErrConflict", func(t *testing.T) {
		if got := mapUniqueViolation("entities_id_lower_key", pairs); !errors.Is(got, store.ErrConflict) {
			t.Errorf("got %v, want ErrConflict", got)
		}
		if got := mapUniqueViolation("entities_pkey", pairs); !errors.Is(got, store.ErrConflict) {
			t.Errorf("got %v, want ErrConflict", got)
		}
	})

	t.Run("owned constraint maps to named property", func(t *testing.T) {
		var up store.UniquePropertyError
		err := mapUniqueViolation(knownName, pairs)
		if !errors.As(err, &up) {
			t.Fatalf("got %v, want UniquePropertyError", err)
		}
		if up.Property != "email" {
			t.Errorf("property = %q, want email", up.Property)
		}
	})

	t.Run("owned but unknown (rolling deploy) degrades to property-less", func(t *testing.T) {
		orphan := uniqueIndexName("gone", "field") // not in pairs
		var up store.UniquePropertyError
		err := mapUniqueViolation(orphan, pairs)
		if !errors.As(err, &up) {
			t.Fatalf("got %v, want UniquePropertyError", err)
		}
		if up.Property != "" {
			t.Errorf("property = %q, want empty (unattributed)", up.Property)
		}
	})
}
