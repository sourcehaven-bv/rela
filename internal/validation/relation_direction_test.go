package validation_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validation"
	"github.com/Sourcehaven-BV/rela/internal/validationgraph"
)

// atlasWorkspace models the shape that motivated direction/target_type: a
// `gaat_over` edge pointing FROM a task or a schedule TO a procedure.
//
// From the procedure there is nothing outgoing to count, and the two source
// types carry different status vocabularies — so neither a bare count nor a
// `status` filter alone can express "this procedure has an open task".
func atlasWorkspace(
	t *testing.T, rule metamodel.ValidationRule, entities []*entity.Entity, rels [][3]string,
) lua.ReadDeps {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"procedure": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "string"},
			}},
			"taak": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "string"},
			}},
			"terugkerend": {Properties: map[string]metamodel.PropertyDef{
				"actief": {Type: "string"},
			}},
		},
		Relations: map[string]metamodel.RelationDef{
			"gaat_over": {
				From: []string{"taak", "terugkerend"},
				To:   []string{"procedure"},
			},
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

// newAtlasSvc wires a Service the way production does, including the real
// adapter — so these tests exercise the query shape, not a stand-in.
func newAtlasSvc(t *testing.T, deps lua.ReadDeps) *validation.Service {
	t.Helper()
	g, err := validationgraph.New(deps.VisibleReader)
	if err != nil {
		t.Fatalf("validationgraph.New: %v", err)
	}
	return validation.New(deps.Meta, deps).WithGraph(g)
}

func proc() *entity.Entity {
	return &entity.Entity{ID: "PROC-1", Type: "procedure", Properties: map[string]any{"status": "vastgesteld"}}
}

// The atlas rule: a procedure must have at least one incoming edge from an
// OPEN taak. The same graph must satisfy or violate it depending only on
// direction and target type — which is the whole point of the two keys.
func TestRelationConstraint_IncomingWithTargetType(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:        "procedure-heeft-open-taak",
		Description: "an adopted procedure needs an open task",
		EntityType:  "procedure",
		Relations: map[string]metamodel.RelationConstraint{
			"gaat_over": {
				Direction:  metamodel.RelationDirectionIncoming,
				TargetType: "taak",
				Where:      []string{"status!=gereed"},
				Min:        new(1),
			},
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
			name:     "no edges at all",
			entities: []*entity.Entity{proc()},
			wantViol: true,
		},
		{
			name: "an open taak satisfies it",
			entities: []*entity.Entity{proc(),
				{ID: "TAAK-1", Type: "taak", Properties: map[string]any{"status": "open"}}},
			rels:     [][3]string{{"TAAK-1", "gaat_over", "PROC-1"}},
			wantViol: false,
		},
		{
			name: "a completed taak does not",
			entities: []*entity.Entity{proc(),
				{ID: "TAAK-1", Type: "taak", Properties: map[string]any{"status": "gereed"}}},
			rels:     [][3]string{{"TAAK-1", "gaat_over", "PROC-1"}},
			wantViol: true,
		},
		{
			// The case a bare count cannot express: an edge EXISTS, but it
			// comes from the wrong type. Without target_type this rule would
			// be satisfied by a schedule.
			name: "a terugkerend does not satisfy a taak constraint",
			entities: []*entity.Entity{proc(),
				{ID: "TR-1", Type: "terugkerend", Properties: map[string]any{"actief": "ja"}}},
			rels:     [][3]string{{"TR-1", "gaat_over", "PROC-1"}},
			wantViol: true,
		},
		{
			name: "one of each: the taak is what counts",
			entities: []*entity.Entity{proc(),
				{ID: "TAAK-1", Type: "taak", Properties: map[string]any{"status": "open"}},
				{ID: "TR-1", Type: "terugkerend", Properties: map[string]any{"actief": "ja"}}},
			rels: [][3]string{
				{"TAAK-1", "gaat_over", "PROC-1"},
				{"TR-1", "gaat_over", "PROC-1"},
			},
			wantViol: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := atlasWorkspace(t, rule, tc.entities, tc.rels)
			res := newAtlasSvc(t, deps).Check(context.Background(), tc.entities, nil)
			if len(res.LoadErrors) != 0 {
				t.Fatalf("unexpected load errors: %v", res.LoadErrors)
			}
			got := len(res.Violations) > 0
			if got != tc.wantViol {
				t.Errorf("wantViol=%v got=%v (%d violations)", tc.wantViol, got, len(res.Violations))
			}
		})
	}
}

// Direction must change WHICH edges are counted, not merely which end of an
// already-selected set is read.
//
// The same graph and the same bound: outgoing from the procedure finds
// nothing (the edges point at it), incoming finds the task. A direction
// implementation that selected the wrong edge set would make these two agree.
func TestRelationConstraint_DirectionSelectsDifferentEdges(t *testing.T) {
	entities := []*entity.Entity{proc(),
		{ID: "TAAK-1", Type: "taak", Properties: map[string]any{"status": "open"}}}
	rels := [][3]string{{"TAAK-1", "gaat_over", "PROC-1"}}

	for _, tc := range []struct {
		name     string
		dir      string
		wantViol bool
	}{
		{"incoming sees the edge", metamodel.RelationDirectionIncoming, false},
		{"outgoing sees nothing", metamodel.RelationDirectionOutgoing, true},
		{"omitted behaves as outgoing", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := metamodel.ValidationRule{
				Name:        "needs-gaat-over",
				Description: "procedure needs a gaat_over edge",
				EntityType:  "procedure",
				Relations: map[string]metamodel.RelationConstraint{
					"gaat_over": {Direction: tc.dir, Min: new(1)},
				},
				Severity: "error",
			}
			deps := atlasWorkspace(t, rule, entities, rels)
			res := newAtlasSvc(t, deps).Check(context.Background(), entities, nil)
			got := len(res.Violations) > 0
			if got != tc.wantViol {
				t.Errorf("direction=%q: wantViol=%v got=%v", tc.dir, tc.wantViol, got)
			}
		})
	}
}

// A `max:` bound with a type filter must still fail closed on an edge whose
// far entity cannot be read.
//
// The type filter is the new way to lose this: an unresolved edge has no type
// to compare, so skipping it "because it does not match" would undercount —
// and an undercount is what a max gate reads as success. The edge has to
// survive as far as the bound's own decision.
func TestRelationConstraint_UnresolvedEdgeCountsUnderMaxWithTargetType(t *testing.T) {
	rule := metamodel.ValidationRule{
		Name:        "no-tasks",
		Description: "procedure must have no incoming taak",
		EntityType:  "procedure",
		Relations: map[string]metamodel.RelationConstraint{
			"gaat_over": {
				Direction:  metamodel.RelationDirectionIncoming,
				TargetType: "taak",
				Max:        new(0),
			},
		},
		Severity: "error",
	}
	entities := []*entity.Entity{proc()}
	// The edge's source is never created, so its far end cannot be read.
	deps := atlasWorkspace(t, rule, entities, [][3]string{{"GONE", "gaat_over", "PROC-1"}})

	res := newAtlasSvc(t, deps).Check(context.Background(), entities, nil)
	if len(res.Violations) == 0 {
		t.Fatal("max gate passed on an unreadable edge; it must fail closed")
	}
}
