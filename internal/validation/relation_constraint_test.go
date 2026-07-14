package validation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

func intPtr(n int) *int { return &n }

// relationWorkspace builds a memstore with a ticket + review-checklist
// schema (plus the given rule) and the supplied entities/relations,
// returning read deps wired to it.
func relationWorkspace(
	t *testing.T, rule metamodel.ValidationRule, entities []*entity.Entity, rels [][3]string,
) lua.ReadDeps {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "string"},
			}},
			"review-checklist": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "string"},
			}},
			"review-response": {Properties: map[string]metamodel.PropertyDef{
				"status":   {Type: "string"},
				"severity": {Type: "string"},
			}},
		},
		Validations: []metamodel.ValidationRule{rule},
	}
	st := memstore.New()
	ctx := context.Background()
	for _, e := range entities {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("create entity %s: %v", e.ID, err)
		}
	}
	for _, r := range rels {
		if _, err := st.CreateRelation(ctx, r[0], r[1], r[2], nil); err != nil {
			t.Fatalf("create relation %v: %v", r, err)
		}
	}
	return lua.ReadDeps{VisibleReader: st, Tracer: tracer.New(st), Meta: meta}
}

func tkt(status string) *entity.Entity {
	return &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"status": status}}
}

func TestRelationConstraint_Min(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:        "done-needs-review",
		Description: "done ticket needs a completed review checklist",
		EntityType:  "ticket",
		When:        []string{"status=done"},
		Relations: map[string]metamodel.RelationConstraint{
			"has-review": {Where: []string{"status=done"}, Min: intPtr(1)},
		},
		Severity: "error",
	}

	tests := []struct {
		name     string
		entities []*entity.Entity
		rels     [][3]string
		wantViol bool
	}{
		{
			name:     "no review checklist at all -> violation",
			entities: []*entity.Entity{tkt("done")},
			wantViol: true,
		},
		{
			name: "review checklist not done -> violation",
			entities: []*entity.Entity{
				tkt("done"),
				{ID: "REV-1", Type: "review-checklist", Properties: map[string]any{"status": "in-progress"}},
			},
			rels:     [][3]string{{"TKT-1", "has-review", "REV-1"}},
			wantViol: true,
		},
		{
			name: "review checklist done -> satisfied",
			entities: []*entity.Entity{
				tkt("done"),
				{ID: "REV-1", Type: "review-checklist", Properties: map[string]any{"status": "done"}},
			},
			rels:     [][3]string{{"TKT-1", "has-review", "REV-1"}},
			wantViol: false,
		},
		{
			name:     "when does not match (not done) -> rule skipped",
			entities: []*entity.Entity{tkt("ready")},
			wantViol: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := relationWorkspace(t, rule, tc.entities, tc.rels)
			svc := New(deps.Meta, deps)
			res := svc.Check(context.Background(), tc.entities, nil)
			gotViol := len(res.Violations) > 0
			if gotViol != tc.wantViol {
				t.Fatalf("wantViol=%v got=%v (%d violations)", tc.wantViol, gotViol, len(res.Violations))
			}
			if tc.wantViol && res.Violations[0].Severity != "error" {
				t.Errorf("severity = %q, want error", res.Violations[0].Severity)
			}
		})
	}
}

func TestRelationConstraint_Max(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:        "no-open-critical",
		Description: "done ticket cannot have open critical review responses",
		EntityType:  "ticket",
		When:        []string{"status=done"},
		Relations: map[string]metamodel.RelationConstraint{
			"has-review-response": {
				Where: []string{"status=open", "severity=critical"},
				Max:   intPtr(0),
			},
		},
		Severity: "error",
	}
	rr := func(id, status, sev string) *entity.Entity {
		return &entity.Entity{ID: id, Type: "review-response",
			Properties: map[string]any{"status": status, "severity": sev}}
	}

	tests := []struct {
		name     string
		entities []*entity.Entity
		rels     [][3]string
		wantViol bool
	}{
		{
			name:     "no responses -> satisfied",
			entities: []*entity.Entity{tkt("done")},
			wantViol: false,
		},
		{
			name: "open critical response -> violation",
			entities: []*entity.Entity{
				tkt("done"),
				rr("RR-1", "open", "critical"),
			},
			rels:     [][3]string{{"TKT-1", "has-review-response", "RR-1"}},
			wantViol: true,
		},
		{
			name: "addressed critical response -> satisfied (where filters it out)",
			entities: []*entity.Entity{
				tkt("done"),
				rr("RR-1", "addressed", "critical"),
			},
			rels:     [][3]string{{"TKT-1", "has-review-response", "RR-1"}},
			wantViol: false,
		},
		{
			name: "open minor response -> satisfied (severity mismatch)",
			entities: []*entity.Entity{
				tkt("done"),
				rr("RR-1", "open", "minor"),
			},
			rels:     [][3]string{{"TKT-1", "has-review-response", "RR-1"}},
			wantViol: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := relationWorkspace(t, rule, tc.entities, tc.rels)
			svc := New(deps.Meta, deps)
			res := svc.Check(context.Background(), tc.entities, nil)
			gotViol := len(res.Violations) > 0
			if gotViol != tc.wantViol {
				t.Fatalf("wantViol=%v got=%v (%d violations)", tc.wantViol, gotViol, len(res.Violations))
			}
		})
	}
}

// TestRelationConstraint_NoStore verifies the check degrades to a no-op
// (rather than panicking) when the service has no store wired.
func TestRelationConstraint_NoStore(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}}},
		},
		Validations: []metamodel.ValidationRule{{
			Name:       "needs-review",
			EntityType: "ticket",
			When:       []string{"status=done"},
			Relations: map[string]metamodel.RelationConstraint{
				"has-review": {Min: intPtr(1)},
			},
			Severity: "error",
		}},
	}
	svc := New(meta, lua.ReadDeps{}) // no VisibleReader
	res := svc.Check(context.Background(), []*entity.Entity{tkt("done")}, nil)
	if len(res.Violations) != 0 {
		t.Fatalf("expected no violations without a reader, got %d", len(res.Violations))
	}
}
