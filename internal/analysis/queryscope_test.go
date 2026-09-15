package analysis_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// scopedTaakMeta declares a type whose DEFAULT query scope hides archived
// rows, so any analysis that honoured the default would stop seeing TAAK-2.
//
// `implements` is required (min 1), which is what turns the scope into a
// correctness question rather than a cosmetic one: an archived task missing
// its required relation is still a broken task.
func scopedTaakMeta() *metamodel.Metamodel {
	one := 1
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Label:      "Taak",
				IDPrefixes: []string{"TAAK-"},
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
				QueryScopes: map[string]string{
					"default": "entity.status ~= 'gearchiveerd'",
				},
			},
			"doel": {Label: "Doel", IDPrefixes: []string{"DOEL-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				From: []string{"taak"}, To: []string{"doel"},
				MinOutgoing: &one,
			},
		},
	}
}

// TestAnalysis_IgnoresQueryScopes is AC6 for the analyze_* family
// (TKT-EVR2TU), and the ticket's headline failure mode.
//
// Analysis answers "what is true about the graph". A query scope answers
// "what should a person see". Folding the second into the first produces the
// `rela validate`-reports-clean-over-unseen-data bug: archive a task with a
// missing required relation and the violation stops being reported, so the
// operator's green check is a statement about a subset they were never told
// about.
//
// One subtest per entry point rather than one aggregate assertion, because
// [analysis.Service] reaches the store through several paths and a regression
// would land on one of them.
func TestAnalysis_IgnoresQueryScopes(t *testing.T) {
	meta := scopedTaakMeta()
	seed := func(s store.Store) {
		// Both violate the min-1 `implements` rule. TAAK-2 is the row a
		// default scope would hide.
		addEntity(s, "TAAK-1", "taak", map[string]any{"status": "todo"})
		addEntity(s, "TAAK-2", "taak", map[string]any{"status": "gearchiveerd"})
	}

	t.Run("cardinality sees archived rows", func(t *testing.T) {
		svc := newServiceWith(t, meta, seed)
		violations, err := svc.CheckCardinality(context.Background(), analysis.Options{})
		if err != nil {
			t.Fatalf("CheckCardinality: %v", err)
		}
		ids := map[string]bool{}
		for _, v := range violations {
			ids[v.EntityID] = true
		}
		if !ids["TAAK-2"] {
			t.Fatalf("archived entity's cardinality violation not reported; archiving a "+
				"broken entity would silence the check. Reported: %v", ids)
		}
		if !ids["TAAK-1"] {
			t.Fatalf("control row missing — the fixture, not the scope, is wrong: %v", ids)
		}
	})

	t.Run("orphans see archived rows", func(t *testing.T) {
		svc := newServiceWith(t, meta, seed)
		orphans := svc.FindOrphansWithScope(context.Background(), analysis.Options{})
		found := map[string]bool{}
		for _, o := range orphans {
			found[o.ID] = true
		}
		if !found["TAAK-2"] {
			t.Fatalf("archived orphan not reported; got %v", found)
		}
	})
}
