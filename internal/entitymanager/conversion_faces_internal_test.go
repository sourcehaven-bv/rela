package entitymanager

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The Tx must not copy a value the read access check did not see.
func TestRequireInputsUnchanged(t *testing.T) {
	d := metamodel.ConversionDef{Fields: map[string]string{"title": "{{source.title}}"}, Content: "fixed"}
	row := func(face entity.Face, title, other string) *entity.Entity {
		e := &entity.Entity{ID: "NOTE-1", Type: "note", Face: face, Properties: map[string]any{}}
		e.Properties["title"], e.Properties["other"] = title, other
		return e
	}
	checked := &conversionPlan{def: d, family: []*entity.Entity{row("draft", "A", "x")}}
	for _, tc := range []struct {
		name    string
		family  []*entity.Entity
		changed bool
	}{
		{"same", []*entity.Entity{row("draft", "A", "x")}, false},
		{"unmapped field changed", []*entity.Entity{row("draft", "A", "y")}, false},
		{"mapped field changed", []*entity.Entity{row("draft", "B", "x")}, true},
		{"face added", []*entity.Entity{row("draft", "A", "x"), row("final", "A", "x")}, true},
		{"face replaced", []*entity.Entity{row("final", "A", "x")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := &conversionRun{checked: checked}
			err := run.requireInputsUnchanged(&conversionPlan{def: d, family: tc.family})
			if got := errors.Is(err, ErrConversionSourceChanged); got != tc.changed {
				t.Fatalf("changed = %v, want %v (err %v)", got, tc.changed, err)
			}
		})
	}
}

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
