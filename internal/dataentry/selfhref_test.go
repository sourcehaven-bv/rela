package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// `_self` must address the ROW the response describes, face included.
//
// It used to be built from the bare id unconditionally, so a client reading
// GUIDE-1@nl got back a pointer to GUIDE-1 — and the ordinary
// GET-`_self` / PATCH-`_self` loop then edited a different content state than
// the one on screen, silently. That is the same wrong-face write the
// `?world=` write refusal exists to prevent (TKT-4Y6CMV / QA F-5).
//
// Since BUG-HC6I2T a faced type has no bare address at all: `bare_face` is
// gone, so every declared face is stored under its own name and every one of
// them serializes with its coordinate. Only a type declaring NO faces keeps
// the unsuffixed href, because its single state has no name to spell.
func TestSelfHref(t *testing.T) {
	for _, tc := range []struct {
		name string
		e    *entity.Entity
		want string
	}{
		{
			// No face is privileged: `en` addresses itself like any other.
			name: "every declared face is addressed by its name",
			e:    &entity.Entity{ID: "GUIDE-1", Type: "guide", Face: entity.Face("en")},
			want: "/api/v1/guides/GUIDE-1@en",
		},
		{
			name: "a second face is addressed the same way",
			e:    &entity.Entity{ID: "GUIDE-1", Type: "guide", Face: entity.Face("nl")},
			want: "/api/v1/guides/GUIDE-1@nl",
		},
		{
			name: "a type with no faces keeps the bare href",
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
			if got := selfHref(plural, tc.e); got != tc.want {
				t.Errorf("selfHref = %q, want %q", got, tc.want)
			}
		})
	}
}
