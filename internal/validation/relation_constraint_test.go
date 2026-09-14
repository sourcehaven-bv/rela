package validation

import (
	"context"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

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
			"has-review": {Where: []string{"status=done"}, Min: new(1)},
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
				Max:   new(0),
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

// TestRelationConstraint_NoReader verifies that a missing reader is
// REPORTED rather than silently satisfying the gate. A wiring error must
// not read as "this entity passed its workflow gates".
func TestRelationConstraint_NoReader(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}}},
		},
		Validations: []metamodel.ValidationRule{{
			Name:       "needs-review",
			EntityType: "ticket",
			When:       []string{"status=done"},
			Relations: map[string]metamodel.RelationConstraint{
				"has-review": {Min: new(1)},
			},
			Severity: "error",
		}},
	}
	svc := New(meta, lua.ReadDeps{}) // no VisibleReader
	res := svc.Check(context.Background(), []*entity.Entity{tkt("done")}, nil)
	if len(res.LoadErrors) == 0 {
		t.Fatal("a missing reader must be reported as a LoadError, not silently pass the gate")
	}
	if len(res.Violations) != 0 {
		t.Errorf("expected no violations (the check could not run), got %d", len(res.Violations))
	}
}

// TestRelationConstraint_UnevaluableTargetFailsClosed pins the polarity of
// the error path. A target that cannot be evaluated — here because the
// `where` filter names a property the target type does not declare, which
// filter.MatchAll reports as a hard error — must NOT be silently dropped
// from the count when Max is set.
//
// Dropping it is how a `max: 0` gate ("a done ticket must have no open
// critical review-responses") silently becomes "always satisfied": the
// count stays at 0 precisely because the targets could not be checked.
// A min gate is conservative when it undercounts, a max gate is not, so
// an unevaluable target counts as matching whenever Max is set.
func TestRelationConstraint_UnevaluableTargetFailsClosed(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:        "no-open-critical",
		Description: "done ticket cannot have open critical review responses",
		EntityType:  "ticket",
		When:        []string{"status=done"},
		Relations: map[string]metamodel.RelationConstraint{
			// `nonexistent` is not declared on review-response, so
			// filter.MatchAll returns an error for every target.
			"has-review-response": {Where: []string{"nonexistent=open"}, Max: new(0)},
		},
		Severity: "error",
	}
	entities := []*entity.Entity{
		tkt("done"),
		{ID: "RR-1", Type: "review-response",
			Properties: map[string]any{"status": "open", "severity": "critical"}},
	}
	deps := relationWorkspace(t, rule, entities, [][3]string{{"TKT-1", "has-review-response", "RR-1"}})
	svc := New(deps.Meta, deps)
	res := svc.Check(context.Background(), entities, nil)
	if len(res.Violations) == 0 {
		t.Fatal("max gate silently passed on an unevaluable target; it must fail closed")
	}
}

// TestRelationConstraint_Boundaries pins the bounds as INCLUSIVE: a count
// exactly equal to min (or to max) satisfies the constraint. Off-by-one
// here would silently re-scope every migrated workflow gate.
func TestRelationConstraint_Boundaries(t *testing.T) {
	rev := func(id string) *entity.Entity {
		return &entity.Entity{ID: id, Type: "review-checklist",
			Properties: map[string]any{"status": "done"}}
	}

	tests := []struct {
		name     string
		c        metamodel.RelationConstraint
		nRels    int
		wantViol bool
	}{
		{name: "count equals min -> satisfied", c: metamodel.RelationConstraint{Min: new(2)}, nRels: 2},
		{name: "count below min -> violation", c: metamodel.RelationConstraint{Min: new(2)}, nRels: 1, wantViol: true},
		{name: "count above min -> satisfied", c: metamodel.RelationConstraint{Min: new(2)}, nRels: 3},
		{name: "count equals max -> satisfied", c: metamodel.RelationConstraint{Max: new(2)}, nRels: 2},
		{name: "count above max -> violation", c: metamodel.RelationConstraint{Max: new(2)}, nRels: 3, wantViol: true},
		{name: "min and max both set, inside range", c: metamodel.RelationConstraint{Min: new(1), Max: new(2)}, nRels: 2},
		{name: "min and max both set, below range",
			c: metamodel.RelationConstraint{Min: new(1), Max: new(2)}, nRels: 0, wantViol: true},
		{name: "min and max both set, above range",
			c: metamodel.RelationConstraint{Min: new(1), Max: new(2)}, nRels: 3, wantViol: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := metamodel.ValidationRule{
				Name:        "bounds",
				Description: "bounds check",
				EntityType:  "ticket",
				When:        []string{"status=done"},
				Relations:   map[string]metamodel.RelationConstraint{"has-review": tc.c},
				Severity:    "error",
			}
			entities := []*entity.Entity{tkt("done")}
			var rels [][3]string
			for i := range tc.nRels {
				id := "REV-" + strconv.Itoa(i)
				entities = append(entities, rev(id))
				rels = append(rels, [3]string{"TKT-1", "has-review", id})
			}
			deps := relationWorkspace(t, rule, entities, rels)
			res := New(deps.Meta, deps).Check(context.Background(), entities, nil)
			if gotViol := len(res.Violations) > 0; gotViol != tc.wantViol {
				t.Fatalf("wantViol=%v got=%v (%d relations, %d violations)",
					tc.wantViol, gotViol, tc.nRels, len(res.Violations))
			}
		})
	}
}

// TestRelationConstraint_MalformedWhereReported is the regression guard for
// the silent-skip class this ticket exists to eliminate: an unparseable
// `where:` must be reported as a LoadError, not quietly counted as zero
// (which would make a max gate pass forever).
func TestRelationConstraint_MalformedWhereReported(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:        "bad-where",
		Description: "malformed where",
		EntityType:  "ticket",
		When:        []string{"status=done"},
		Relations: map[string]metamodel.RelationConstraint{
			"has-review": {Where: []string{"not a filter at all"}, Max: new(0)},
		},
		Severity: "error",
	}
	entities := []*entity.Entity{tkt("done")}
	deps := relationWorkspace(t, rule, entities, nil)
	res := New(deps.Meta, deps).Check(context.Background(), entities, nil)
	if len(res.LoadErrors) == 0 {
		t.Fatal("a malformed where filter must be reported as a LoadError")
	}
}
