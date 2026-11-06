package entitymanager_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var errMoveMetaDenied = errors.New("relation meta write denied")

// sourceMetaGate refuses a relation meta write of denyKey on an edge whose
// source is in denySources, and records every source it judged.
type sourceMetaGate struct {
	recordingGate
	denyKey     string
	denySources map[string]bool
	judged      []string
}

func (g *sourceMetaGate) CheckRelationMetaWrite(
	_ context.Context, source *entity.Entity, _ string, set map[string]any, unset []string,
) error {
	g.judged = append(g.judged, source.ID)
	if !g.denySources[source.ID] {
		return nil
	}
	if _, ok := set[g.denyKey]; ok || slices.Contains(unset, g.denyKey) {
		return errMoveMetaDenied
	}
	return nil
}

// TestMoveRelation_FieldGateJudgesOrderKey pins BUG-FQZ5Q1L: a position
// move on a field-gated handle asks the relation meta gate about the order
// key it writes, for the moved edge's source and, on the incoming side, for
// every other source whose edge the move may rewrite. A refusal writes
// nothing; a move the gate allows swaps the two edges.
func TestMoveRelation_FieldGateJudgesOrderKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	outgoing := []entity.RelationKey{
		{From: "TASK-1", Type: "precedes", To: "TASK-2"},
		{From: "TASK-1", Type: "precedes", To: "TASK-3"},
	}
	incoming := []entity.RelationKey{
		{From: "TASK-1", Type: "precedes", To: "TASK-3"},
		{From: "TASK-2", Type: "precedes", To: "TASK-3"},
	}

	tests := []struct {
		name      string
		incoming  bool
		prop      string
		edges     []entity.RelationKey
		denied    string
		wantJudge []string
	}{
		{
			name: "outgoing move refused for the moved edge's source", prop: metamodel.OrderPropertyOut,
			edges: outgoing, denied: "TASK-1", wantJudge: []string{"TASK-1"},
		},
		{
			name: "incoming move refused for a sibling's source", incoming: true, prop: metamodel.OrderPropertyIn,
			edges: incoming, denied: "TASK-2", wantJudge: []string{"TASK-1", "TASK-2"},
		},
		{
			name: "outgoing move the gate allows", prop: metamodel.OrderPropertyOut,
			edges: outgoing, wantJudge: []string{"TASK-1"},
		},
		{
			name: "incoming move the gate allows", incoming: true, prop: metamodel.OrderPropertyIn,
			edges: incoming, wantJudge: []string{"TASK-1", "TASK-2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gate := &sourceMetaGate{denyKey: tc.prop, denySources: map[string]bool{tc.denied: true}}
			mgr, st := newPatchManager(t, gate)
			for _, id := range []string{"TASK-1", "TASK-2", "TASK-3"} {
				task := entity.New(id, "task")
				task.Properties = map[string]any{"title": id}
				if err := st.CreateEntity(ctx, task); err != nil {
					t.Fatalf("seed %s: %v", id, err)
				}
			}
			for i, key := range tc.edges {
				props := map[string]any{tc.prop: float64(i + 1)}
				if _, err := st.CreateRelation(ctx, key, &store.RelationData{Properties: props}); err != nil {
					t.Fatalf("seed edge %v: %v", key, err)
				}
			}

			_, err := mgr.UpdateRelation(ctx, tc.edges[0], entity.RelationOptions{
				Position: &entity.OrderPosition{Step: 1, Incoming: tc.incoming},
			})
			refused := tc.denied != ""
			switch {
			case refused && !errors.Is(err, errMoveMetaDenied):
				t.Fatalf("move err = %v, want the gate's refusal", err)
			case !refused && err != nil:
				t.Fatalf("move: %v", err)
			}
			if !slices.Equal(gate.judged, tc.wantJudge) {
				t.Errorf("gate judged %v, want %v", gate.judged, tc.wantJudge)
			}

			first, err := st.GetRelation(ctx, tc.edges[0])
			if err != nil {
				t.Fatalf("get %v: %v", tc.edges[0], err)
			}
			second, err := st.GetRelation(ctx, tc.edges[1])
			if err != nil {
				t.Fatalf("get %v: %v", tc.edges[1], err)
			}
			before := first.Properties[tc.prop].(float64) < second.Properties[tc.prop].(float64)
			if before != refused {
				t.Errorf("%s: moved edge %v, sibling %v; refused=%v", tc.prop,
					first.Properties[tc.prop], second.Properties[tc.prop], refused)
			}
		})
	}
}

// TestRenumberAfterUpdate_FieldGateJudgesSiblingSources pins the renumber
// half of BUG-FQZ5Q1L: an explicit incoming order write that collapses the
// gap renumbers every edge into the target, which rewrites other sources'
// order values as a move's densify does. On a field-gated handle it runs only
// when those sources pass the gate for the order key; otherwise the update
// stands and the renumber is skipped.
func TestRenumberAfterUpdate_FieldGateJudgesSiblingSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	prop := metamodel.OrderPropertyIn
	mine := entity.RelationKey{From: "TASK-1", Type: "precedes", To: "TASK-3"}
	theirs := entity.RelationKey{From: "TASK-2", Type: "precedes", To: "TASK-3"}

	for _, deny := range []bool{true, false} {
		t.Run(map[bool]string{true: "sibling source refuses", false: "sibling source allows"}[deny], func(t *testing.T) {
			t.Parallel()
			gate := &sourceMetaGate{denyKey: prop, denySources: map[string]bool{"TASK-2": deny}}
			mgr, st := newPatchManager(t, gate)
			for _, id := range []string{"TASK-1", "TASK-2", "TASK-3"} {
				task := entity.New(id, "task")
				task.Properties = map[string]any{"title": id}
				if err := st.CreateEntity(ctx, task); err != nil {
					t.Fatalf("seed %s: %v", id, err)
				}
			}
			for i, key := range []entity.RelationKey{mine, theirs} {
				props := map[string]any{prop: float64(i + 1)}
				if _, err := st.CreateRelation(ctx, key, &store.RelationData{Properties: props}); err != nil {
					t.Fatalf("seed edge %v: %v", key, err)
				}
			}

			collapsed := 2 + entitymanager.OrderCollapseThreshold/2
			if _, err := mgr.UpdateRelation(ctx, mine, entity.RelationOptions{
				Properties: map[string]any{prop: collapsed},
			}); err != nil {
				t.Fatalf("UpdateRelation: %v", err)
			}
			rel, err := st.GetRelation(ctx, theirs)
			if err != nil {
				t.Fatalf("get %v: %v", theirs, err)
			}
			renumbered := rel.Properties[prop] != 2.0
			if renumbered == deny {
				t.Errorf("sibling %s = %v; renumbered=%v with the gate refusing=%v",
					prop, rel.Properties[prop], renumbered, deny)
			}
		})
	}
}
