package workspace

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestFindOrphansWithScope(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document"},
		},
	}

	// Create entities
	g.AddNode(&model.Entity{ID: "DOC-001", Type: "doc"})
	g.AddNode(&model.Entity{ID: "DOC-002", Type: "doc"})
	g.AddNode(&model.Entity{ID: "DOC-003", Type: "doc"})

	// Link DOC-001 to DOC-002
	g.AddEdge(&model.Relation{From: "DOC-001", Type: "refs", To: "DOC-002"})

	ws := NewForTest(g, meta)

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
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document"},
		},
	}

	g.AddNode(&model.Entity{
		ID:         "DOC-001",
		Type:       "doc",
		Properties: map[string]interface{}{"title": "Test Document"},
	})
	g.AddNode(&model.Entity{
		ID:         "DOC-002",
		Type:       "doc",
		Properties: map[string]interface{}{"title": "test document"}, // Same normalized
	})
	g.AddNode(&model.Entity{
		ID:         "DOC-003",
		Type:       "doc",
		Properties: map[string]interface{}{"title": "Different"},
	})

	ws := NewForTest(g, meta)

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
	g := graph.New()
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

	g.AddNode(&model.Entity{ID: "TKT-001", Type: "ticket"})
	g.AddNode(&model.Entity{ID: "TKT-002", Type: "ticket"})
	g.AddNode(&model.Entity{ID: "CON-001", Type: "concept"})

	// Only TKT-001 has the required relation
	g.AddEdge(&model.Relation{From: "TKT-001", Type: "affects", To: "CON-001"})

	ws := NewForTest(g, meta)

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

func TestValidateProperties(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:      "Ticket",
				IDPrefixes: []string{"TKT-"},
				Properties: map[string]metamodel.PropertyDef{
					"status": {
						Type:     "enum",
						Required: true,
						Values:   []string{"open", "closed"},
					},
				},
			},
		},
	}

	g.AddNode(&model.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"status": "open"},
	})
	g.AddNode(&model.Entity{
		ID:         "TKT-002",
		Type:       "ticket",
		Properties: map[string]interface{}{"status": "invalid"},
	})

	ws := NewForTest(g, meta)

	t.Run("finds property errors", func(t *testing.T) {
		errs := ws.ValidateProperties(AnalyzeOptions{})
		if len(errs) != 1 {
			t.Errorf("got %d entities with errors, want 1", len(errs))
		}
		if len(errs) > 0 && errs[0].EntityID != "TKT-002" {
			t.Errorf("error entity = %s, want TKT-002", errs[0].EntityID)
		}
	})

	t.Run("scope filters errors", func(t *testing.T) {
		errs := ws.ValidateProperties(AnalyzeOptions{
			Scope: map[string]bool{"TKT-001": true},
		})
		if len(errs) != 0 {
			t.Errorf("got %d entities with errors, want 0", len(errs))
		}
	})
}

func TestAnalyzeAll(t *testing.T) {
	g := graph.New()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document", IDPrefixes: []string{"DOC-"}},
		},
	}

	// Create one orphan
	g.AddNode(&model.Entity{ID: "DOC-001", Type: "doc"})

	ws := NewForTest(g, meta)

	summary := ws.AnalyzeAll(AnalyzeOptions{})
	if summary.Orphans != 1 {
		t.Errorf("Orphans = %d, want 1", summary.Orphans)
	}
}

func TestFilterByScope(t *testing.T) {
	entities := []*model.Entity{
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

func TestInScope(t *testing.T) {
	t.Run("nil scope returns true", func(t *testing.T) {
		if !inScope("any", nil) {
			t.Error("inScope(any, nil) = false, want true")
		}
	})

	t.Run("in scope returns true", func(t *testing.T) {
		if !inScope("A", map[string]bool{"A": true}) {
			t.Error("inScope(A, {A:true}) = false, want true")
		}
	})

	t.Run("not in scope returns false", func(t *testing.T) {
		if inScope("B", map[string]bool{"A": true}) {
			t.Error("inScope(B, {A:true}) = true, want false")
		}
	})

	t.Run("key exists with false value returns true", func(t *testing.T) {
		// This tests that we check key existence, not value
		if !inScope("A", map[string]bool{"A": false}) {
			t.Error("inScope(A, {A:false}) = false, want true")
		}
	})
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Test Document", "test document"},
		{"  Spaces  ", "spaces"},
		{"Multiple   Spaces", "multiple spaces"},
		{"UPPERCASE", "uppercase"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeTitle(tt.input)
		if got != tt.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
