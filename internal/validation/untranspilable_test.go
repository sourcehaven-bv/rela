package validation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestValidation_UntranspilableThenFallsBackToFilter pins RR-FI4DYL: a
// Then: clause the predicate transpiler can't express (fuzzy-with-
// wildcard on a string property) must fall back to filter.MatchAll, NOT
// become a forced violation on every entity. An entity that satisfies
// the fuzzy Then produces no violation.
func TestValidation_UntranspilableThenFallsBackToFilter(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: metamodel.PropertyTypeString},
			}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "title-fuzzy",
				EntityType:  "ticket",
				Description: "title should fuzzily match 'urgent*'",
				// fuzzy-with-wildcard: FromFilter refuses to transpile this.
				Then:     []string{"title~urgent*"},
				Severity: "error",
			},
		},
	}

	entities := []*entity.Entity{
		// Satisfies the fuzzy Then (title starts ~ "urgent"): no violation.
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "urgentish task"}},
	}

	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.Violations) != 0 {
		t.Fatalf("got %d violations, want 0: an untranspilable Then must fall back to "+
			"filter.MatchAll, not force a violation; %+v", len(result.Violations), result.Violations)
	}
}
