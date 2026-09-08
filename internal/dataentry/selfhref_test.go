package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// `_self` must address the ROW the response describes, face included.
//
// It used to be built from the bare id unconditionally, so a client reading
// GUIDE-1@nl got back a pointer to GUIDE-1 — and the ordinary
// GET-`_self` / PATCH-`_self` loop then edited a different content state than
// the one on screen, silently. That is the same wrong-face write the
// `?world=` write refusal exists to prevent (TKT-4Y6CMV / QA F-5).
func TestSelfHref(t *testing.T) {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"guide": {
			BareFace: "en",
			Faces:    map[string]metamodel.FaceDef{"en": {}, "nl": {}},
		},
		"plain": {},
	}}

	for _, tc := range []struct {
		name string
		e    *entity.Entity
		want string
	}{
		{
			// The bare face IS the bare id; appending its declared name would
			// break every existing client for no gain.
			name: "bare face keeps the bare href",
			e:    &entity.Entity{ID: "GUIDE-1", Type: "guide"},
			want: "/api/v1/guides/GUIDE-1",
		},
		{
			name: "non-bare face is addressed by its declared name",
			e:    &entity.Entity{ID: "GUIDE-1", Type: "guide", Face: entity.Face("nl")},
			want: "/api/v1/guides/GUIDE-1@nl",
		},
		{
			name: "a type with no faces is unaffected",
			e:    &entity.Entity{ID: "P-1", Type: "plain"},
			want: "/api/v1/plains/P-1",
		},
		{
			// An undeclared stored coordinate still round-trips to its own
			// row rather than silently pointing at a sibling.
			name: "undeclared stored face keeps its coordinate",
			e:    &entity.Entity{ID: "GUIDE-1", Type: "guide", Face: entity.Face("fr")},
			want: "/api/v1/guides/GUIDE-1@fr",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plural := "guides"
			if tc.e.Type == "plain" {
				plural = "plains"
			}
			if got := selfHref(plural, tc.e, meta); got != tc.want {
				t.Errorf("selfHref = %q, want %q", got, tc.want)
			}
		})
	}
}
