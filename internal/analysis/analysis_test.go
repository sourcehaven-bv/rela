package analysis_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// addEntity / addRelation: terse seed helpers that panic on error.
// Used widely below; tests stay focused on the behavior under check.

func addEntity(s store.Store, id, entityType string, props map[string]interface{}) {
	if err := s.CreateEntity(context.Background(), &entity.Entity{
		ID: id, Type: entityType, Properties: props,
	}); err != nil {
		panic(err)
	}
}

func addRelation(s store.Store, from, relType, to string) {
	if _, err := s.CreateRelation(context.Background(), from, relType, to, nil); err != nil {
		panic(err)
	}
}

// newServiceWith builds a Service backed by a fresh memstore. Seed
// runs before tracer is captured so tracer observes the final state.
// LuaReadDeps is wired so RunValidations can construct a validation
// service even when tests don't exercise Lua.
func newServiceWith(t *testing.T, meta *metamodel.Metamodel, seed func(store.Store)) *analysis.Service {
	t.Helper()
	st := memstore.New()
	if seed != nil {
		seed(st)
	}
	tr := tracer.New(st)
	svc, err := analysis.New(analysis.Deps{
		Store:  st,
		Meta:   meta,
		Tracer: tr,
		LuaReadDeps: lua.ReadDeps{
			Store:  st,
			Tracer: tr,
			Meta:   meta,
		},
	})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}
	return svc
}

func TestFindOrphansWithScope(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Label: "Document"},
		},
	}

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "DOC-001", "doc", nil)
		addEntity(s, "DOC-002", "doc", nil)
		addEntity(s, "DOC-003", "doc", nil)
		addRelation(s, "DOC-001", "refs", "DOC-002")
	})

	t.Run("no scope", func(t *testing.T) {
		orphans := svc.FindOrphansWithScope(context.Background(), analysis.Options{})
		if len(orphans) != 1 {
			t.Errorf("got %d orphans, want 1", len(orphans))
		}
		if len(orphans) > 0 && orphans[0].ID != "DOC-003" {
			t.Errorf("orphan = %s, want DOC-003", orphans[0].ID)
		}
	})

	t.Run("with scope including orphan", func(t *testing.T) {
		orphans := svc.FindOrphansWithScope(context.Background(), analysis.Options{
			Scope: map[string]bool{"DOC-003": true},
		})
		if len(orphans) != 1 {
			t.Errorf("got %d orphans, want 1", len(orphans))
		}
	})

	t.Run("with scope excluding orphan", func(t *testing.T) {
		orphans := svc.FindOrphansWithScope(context.Background(), analysis.Options{
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

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "DOC-001", "doc", map[string]interface{}{"title": "Test Document"})
		addEntity(s, "DOC-002", "doc", map[string]interface{}{"title": "test document"})
		addEntity(s, "DOC-003", "doc", map[string]interface{}{"title": "Different"})
	})

	t.Run("finds duplicates", func(t *testing.T) {
		dups := svc.FindDuplicates(context.Background(), analysis.Options{})
		if len(dups) != 1 {
			t.Errorf("got %d duplicate groups, want 1", len(dups))
		}
		if len(dups) > 0 && len(dups[0].Entities) != 2 {
			t.Errorf("duplicate group has %d entities, want 2", len(dups[0].Entities))
		}
	})

	t.Run("scope filters duplicates", func(t *testing.T) {
		dups := svc.FindDuplicates(context.Background(), analysis.Options{
			Scope: map[string]bool{"DOC-001": true},
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
				MinOutgoing: &minOne,
			},
		},
	}

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "TKT-001", "ticket", nil)
		addEntity(s, "TKT-002", "ticket", nil)
		addEntity(s, "CON-001", "concept", nil)
		addRelation(s, "TKT-001", "affects", "CON-001")
	})

	t.Run("finds violations", func(t *testing.T) {
		violations := svc.CheckCardinality(context.Background(), analysis.Options{})
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
		violations := svc.CheckCardinality(context.Background(), analysis.Options{
			Scope: map[string]bool{"TKT-001": true},
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

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "DOC-001", "doc", nil)
	})

	summary := svc.AnalyzeAll(context.Background(), analysis.Options{})
	if summary.Orphans != 1 {
		t.Errorf("Orphans = %d, want 1", summary.Orphans)
	}
}

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

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "TKT-001", "ticket", map[string]interface{}{"status": "in-progress"})
	})

	violations := svc.RunValidations(context.Background(), analysis.Options{}).Violations
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].EntityID != "TKT-001" {
		t.Errorf("violation entity = %s, want TKT-001", violations[0].EntityID)
	}
}

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

	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "TKT-001", "ticket", map[string]interface{}{"status": "bad"})
		addEntity(s, "BUG-001", "bug", map[string]interface{}{"status": "bad"})
	})

	t.Run("filter by rule name", func(t *testing.T) {
		violations := svc.RunValidationsFiltered(
			context.Background(), analysis.Options{}, []analysis.ValidationFilter{{RuleName: "ticket-rule"}},
		).Violations
		if len(violations) != 1 || violations[0].RuleName != "ticket-rule" {
			t.Errorf("got %#v, want one ticket-rule violation", violations)
		}
	})

	t.Run("filter by entity type", func(t *testing.T) {
		violations := svc.RunValidationsFiltered(
			context.Background(), analysis.Options{}, []analysis.ValidationFilter{{EntityType: "bug"}},
		).Violations
		if len(violations) != 1 || violations[0].RuleName != "bug-rule" {
			t.Errorf("got %#v, want one bug-rule violation", violations)
		}
	})
}

func TestService_New_RejectsNilDeps(t *testing.T) {
	// Zero-value Metamodel and a real memstore are passed only to
	// advance past earlier nil-checks. None are dereferenced before
	// the next nil-check fires.
	meta := &metamodel.Metamodel{}
	st := memstore.New()
	tr := tracer.New(st)

	cases := []struct {
		name string
		d    analysis.Deps
		want string
	}{
		{"nil store", analysis.Deps{Meta: meta, Tracer: tr}, "Store is required"},
		{"nil meta", analysis.Deps{Store: st, Tracer: tr}, "Meta is required"},
		{"nil tracer", analysis.Deps{Store: st, Meta: meta}, "Tracer is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := analysis.New(tc.d)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
