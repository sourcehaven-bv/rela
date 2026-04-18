package workspace

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// seedStore builds a fresh memstore and applies the given seed function.
// Tests use it in place of the old graph.AddNode/AddEdge pattern.
func seedStore(t *testing.T, seed func(store.Store)) store.Store {
	t.Helper()
	s := memstore.New()
	if seed != nil {
		seed(s)
	}
	return s
}

// addEntity is a tiny helper that panics on error so tests stay terse.
func addEntity(s store.Store, id, entityType string, props map[string]interface{}) {
	ctx := context.Background()
	if err := s.CreateEntity(ctx, &entity.Entity{
		ID:         id,
		Type:       entityType,
		Properties: props,
	}); err != nil {
		panic(err)
	}
}

// addRelation is a tiny helper that panics on error so tests stay terse.
func addRelation(s store.Store, from, relType, to string) {
	ctx := context.Background()
	if _, err := s.CreateRelation(ctx, from, relType, to, nil); err != nil {
		panic(err)
	}
}

func TestFindOrphansWithScope(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document"},
		},
	}

	s := seedStore(t, func(s store.Store) {
		addEntity(s, "DOC-001", "doc", nil)
		addEntity(s, "DOC-002", "doc", nil)
		addEntity(s, "DOC-003", "doc", nil)
		addRelation(s, "DOC-001", "refs", "DOC-002")
	})
	ws := NewForTest(meta, WithTestStore(s))

	t.Run("no scope", func(t *testing.T) {
		orphans := ws.FindOrphansWithScope(AnalyzeOptions{})
		// DOC-003 is orphan
		if len(orphans) != 1 {
			t.Errorf("got %d orphans, want 1", len(orphans))
		}
		if len(orphans) > 0 && orphans[0].ID != "DOC-003" {
			t.Errorf("orphan = %s, want DOC-003", orphans[0].ID)
		}
	})

	t.Run("with scope including orphan", func(t *testing.T) {
		orphans := ws.FindOrphansWithScope(AnalyzeOptions{
			Scope: map[string]bool{"DOC-003": true},
		})
		if len(orphans) != 1 {
			t.Errorf("got %d orphans, want 1", len(orphans))
		}
	})

	t.Run("with scope excluding orphan", func(t *testing.T) {
		orphans := ws.FindOrphansWithScope(AnalyzeOptions{
			Scope: map[string]bool{"DOC-001": true, "DOC-002": true},
		})
		if len(orphans) != 0 {
			t.Errorf("got %d orphans, want 0", len(orphans))
		}
	})
}

func TestFindDuplicates(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document"},
		},
	}

	s := seedStore(t, func(s store.Store) {
		addEntity(s, "DOC-001", "doc", map[string]interface{}{"title": "Test Document"})
		addEntity(s, "DOC-002", "doc", map[string]interface{}{"title": "test document"}) // Same normalized
		addEntity(s, "DOC-003", "doc", map[string]interface{}{"title": "Different"})
	})
	ws := NewForTest(meta, WithTestStore(s))

	t.Run("finds duplicates", func(t *testing.T) {
		dups := ws.FindDuplicates(AnalyzeOptions{})
		if len(dups) != 1 {
			t.Errorf("got %d duplicate groups, want 1", len(dups))
		}
		if len(dups) > 0 && len(dups[0].Entities) != 2 {
			t.Errorf("duplicate group has %d entities, want 2", len(dups[0].Entities))
		}
	})

	t.Run("scope filters duplicates", func(t *testing.T) {
		dups := ws.FindDuplicates(AnalyzeOptions{
			Scope: map[string]bool{"DOC-001": true}, // Only one of the duplicates
		})
		if len(dups) != 0 {
			t.Errorf("got %d duplicate groups, want 0", len(dups))
		}
	})
}

func TestCheckCardinality(t *testing.T) {
	minOne := 1
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefixes: []string{"TKT-"}},
			"concept": {Label: "Concept", IDPrefixes: []string{"CON-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects": {
				Label:       "affects",
				From:        []string{"ticket"},
				To:          []string{"concept"},
				MinOutgoing: &minOne, // Every ticket must affect at least 1 concept
			},
		},
	}

	s := seedStore(t, func(s store.Store) {
		addEntity(s, "TKT-001", "ticket", nil)
		addEntity(s, "TKT-002", "ticket", nil)
		addEntity(s, "CON-001", "concept", nil)
		// Only TKT-001 has the required relation
		addRelation(s, "TKT-001", "affects", "CON-001")
	})
	ws := NewForTest(meta, WithTestStore(s))

	t.Run("finds violations", func(t *testing.T) {
		violations := ws.CheckCardinality(AnalyzeOptions{})
		if len(violations) != 1 {
			t.Errorf("got %d violations, want 1", len(violations))
		}
		if len(violations) > 0 {
			if violations[0].EntityID != "TKT-002" {
				t.Errorf("violation entity = %s, want TKT-002", violations[0].EntityID)
			}
			if violations[0].Constraint != "min_outgoing" {
				t.Errorf("constraint = %s, want min_outgoing", violations[0].Constraint)
			}
		}
	})

	t.Run("scope filters violations", func(t *testing.T) {
		violations := ws.CheckCardinality(AnalyzeOptions{
			Scope: map[string]bool{"TKT-001": true}, // Only the compliant ticket
		})
		if len(violations) != 0 {
			t.Errorf("got %d violations, want 0", len(violations))
		}
	})
}

func TestAnalyzeAll(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document", IDPrefixes: []string{"DOC-"}},
		},
	}

	// Create one orphan
	s := seedStore(t, func(s store.Store) {
		addEntity(s, "DOC-001", "doc", nil)
	})
	ws := NewForTest(meta, WithTestStore(s))

	summary := ws.AnalyzeAll(AnalyzeOptions{})
	if summary.Orphans != 1 {
		t.Errorf("Orphans = %d, want 1", summary.Orphans)
	}
}

// TestRunValidations exercises the custom-rule validation pipeline.
// Moved here from internal/cli/validate_test.go so the correctness
// check lives with the workspace logic it covers.
func TestRunValidations(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "string"},
					"assignee": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "in-progress-needs-assignee",
				Description: "In-progress tickets must have an assignee",
				EntityType:  "ticket",
				When:        []string{"status=in-progress"},
				Then:        []string{"assignee!="},
				Severity:    "error",
			},
		},
	}
	meta.InitAliases()

	s := seedStore(t, func(s store.Store) {
		addEntity(s, "TKT-001", "ticket", map[string]interface{}{"status": "in-progress"})
	})
	ws := NewForTest(meta, WithTestStore(s))

	violations := ws.RunValidations(AnalyzeOptions{})
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].EntityID != "TKT-001" {
		t.Errorf("violation entity = %s, want TKT-001", violations[0].EntityID)
	}
}

// TestRunValidationsFiltered confirms the filter-by-rule-name and
// filter-by-entity-type paths. Moved here from the CLI tests.
func TestRunValidationsFiltered(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:      "Ticket",
				IDPrefix:   "TKT-",
				Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}},
			},
			"bug": {
				Label:      "Bug",
				IDPrefix:   "BUG-",
				Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "ticket-rule",
				EntityType: "ticket",
				When:       []string{"status=bad"},
				Then:       []string{"status!=bad"},
				Severity:   "error",
			},
			{
				Name:       "bug-rule",
				EntityType: "bug",
				When:       []string{"status=bad"},
				Then:       []string{"status!=bad"},
				Severity:   "error",
			},
		},
	}
	meta.InitAliases()

	s := seedStore(t, func(s store.Store) {
		addEntity(s, "TKT-001", "ticket", map[string]interface{}{"status": "bad"})
		addEntity(s, "BUG-001", "bug", map[string]interface{}{"status": "bad"})
	})
	ws := NewForTest(meta, WithTestStore(s))

	// Filter by rule name.
	violations := ws.RunValidationsFiltered(AnalyzeOptions{}, []ValidationFilter{{RuleName: "ticket-rule"}})
	if len(violations) != 1 || violations[0].RuleName != "ticket-rule" {
		t.Errorf("rule-name filter: got %#v, want one ticket-rule violation", violations)
	}

	// Filter by entity type.
	violations = ws.RunValidationsFiltered(AnalyzeOptions{}, []ValidationFilter{{EntityType: "bug"}})
	if len(violations) != 1 || violations[0].RuleName != "bug-rule" {
		t.Errorf("entity-type filter: got %#v, want one bug-rule violation", violations)
	}
}

func TestFilterByScope(t *testing.T) {
	entities := []*entity.Entity{
		{ID: "A"},
		{ID: "B"},
		{ID: "C"},
	}

	t.Run("nil scope returns all", func(t *testing.T) {
		result := filterByScope(entities, nil)
		if len(result) != 3 {
			t.Errorf("got %d entities, want 3", len(result))
		}
	})

	t.Run("scope filters entities", func(t *testing.T) {
		result := filterByScope(entities, map[string]bool{"A": true, "C": true})
		if len(result) != 2 {
			t.Errorf("got %d entities, want 2", len(result))
		}
	})

	t.Run("empty scope returns none", func(t *testing.T) {
		result := filterByScope(entities, map[string]bool{})
		if len(result) != 0 {
			t.Errorf("got %d entities, want 0", len(result))
		}
	})
}
