package store_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestVersionOf_DistinguishesValueTypes pins that the token is not type-blind.
//
// fmt's %v renders int64(1), float64(1) and "1" identically, as it does true
// and "true", or a []string and its bracketed form. Hashing the rendering alone
// gave two entities differing only in a property's TYPE the same token — so a
// concurrent CAS that ought to conflict would silently succeed and the losing
// writer's change would vanish. That is the precise failure the token exists to
// prevent, and it is reachable: nothing coerces property types on the write
// path, so a script writing 1 where a form wrote "1" is ordinary.
func TestVersionOf_DistinguishesValueTypes(t *testing.T) {
	t.Parallel()

	versionWith := func(v any) store.EntityVersion {
		e := entity.New("E-1", "thing")
		e.Properties = map[string]any{"p": v}
		return store.VersionOf(e)
	}

	tests := []struct {
		name string
		a, b any
	}{
		{"int64 vs its string form", int64(1), "1"},
		{"float64 vs int64", float64(1), int64(1)},
		{"int vs float", 1, 1.0},
		{"bool vs its string form", true, "true"},
		{"list vs its bracketed form", []string{"a", "b"}, "[a b]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if versionWith(tc.a) == versionWith(tc.b) {
				t.Errorf("%#v and %#v produce the same version; a type-only change "+
					"would pass a CAS that should conflict", tc.a, tc.b)
			}
		})
	}
}

// TestVersionOf_StableAcrossInsertionOrder pins the other half: the token must
// NOT change for content that is genuinely equal. Go map iteration is
// randomised, so an unsorted hash would produce spurious conflicts on an
// untouched entity.
func TestVersionOf_StableAcrossInsertionOrder(t *testing.T) {
	t.Parallel()

	a := entity.New("E-1", "thing")
	a.Properties = map[string]any{"alpha": "1", "beta": "2", "gamma": "3"}
	b := entity.New("E-1", "thing")
	b.Properties = map[string]any{"gamma": "3", "beta": "2", "alpha": "1"}

	for i := range 100 {
		if got, want := store.VersionOf(b), store.VersionOf(a); got != want {
			t.Fatalf("iteration %d: version differs for equal content (%q vs %q)", i, got, want)
		}
	}
}
