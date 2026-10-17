package entitymanager

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Two source faces must never land on one target face, whatever order the
// store returns the family rows in.
func TestResolveConversionFacesRejectsSharedTarget(t *testing.T) {
	d := metamodel.ConversionDef{Faces: map[string][]string{
		"draft":     {"approved"},
		"published": {"approved"},
	}}
	draft := &entity.Entity{ID: "NOTE-1", Type: "note", Face: "draft"}
	published := &entity.Entity{ID: "NOTE-1", Type: "note", Face: "published"}
	for _, tc := range []struct {
		name   string
		family []*entity.Entity
	}{
		{"draft first", []*entity.Entity{draft, published}},
		{"published first", []*entity.Entity{published, draft}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, required, invalid := resolveConversionFaces(d, tc.family, nil)
			if len(required) != 0 {
				t.Fatalf("required = %v", required)
			}
			if len(invalid) != 1 {
				t.Fatalf("invalid = %v, want one shared-target error", invalid)
			}
			if len(result) != 1 {
				t.Fatalf("result = %v, want only the first face mapped", result)
			}
		})
	}
}
