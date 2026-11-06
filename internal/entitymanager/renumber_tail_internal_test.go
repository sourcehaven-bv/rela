package entitymanager

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestMaybeRenumberSide_TwoTailsOfOneTriple pins that renumber treats two
// tails of one triple as two siblings (TKT-KQXVF7). A triple-keyed plan
// collapsed them: both sorted slots resolved to one edge, which was written
// twice while the other kept its collapsed value.
func TestMaybeRenumberSide_TwoTailsOfOneTriple(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "POL-1", Type: "policy", Face: "published"},
		{ID: "CTL-1", Type: "control"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	prop := metamodel.OrderPropertyIn
	keys := []entity.RelationKey{
		{From: "POL-1", FromFace: "draft", Type: "cites", To: "CTL-1"},
		{From: "POL-1", FromFace: "published", Type: "cites", To: "CTL-1"},
	}
	// Two values closer than the collapse threshold force a renumber.
	for i, k := range keys {
		data := &store.RelationData{Properties: map[string]any{prop: 1.0 + float64(i)*1e-12}}
		if _, err := st.CreateRelation(ctx, k, data); err != nil {
			t.Fatal(err)
		}
	}

	updated, err := maybeRenumberSide(ctx, st, store.RelationQuery{To: "CTL-1", Type: "cites"}, prop, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 {
		t.Fatalf("updated %d edges, want 1 (the first already holds 1)", len(updated))
	}
	got := map[float64]entity.RelationKey{}
	for _, k := range keys {
		r, err := st.GetRelation(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		v, _ := FiniteOrder(r.Properties[prop])
		got[v] = k
	}
	if got[1] != keys[0] || got[2] != keys[1] {
		t.Errorf("order values = %v, want draft=1, published=2", got)
	}
}

// TestMaybeRenumberSide_SkipsUnjudgedSibling pins that the renumber's
// transaction rewrites nothing when it lists an edge the relation meta gate
// did not judge before it (BUG-FQZ5Q1L): an edge created between the check
// and the transaction must not be rewritten unjudged.
func TestMaybeRenumberSide_SkipsUnjudgedSibling(t *testing.T) {
	ctx := context.Background()
	prop := metamodel.OrderPropertyIn
	q := store.RelationQuery{To: "T-1", Type: "precedes"}
	keys := []entity.RelationKey{
		{From: "A-1", Type: "precedes", To: "T-1"},
		{From: "B-1", Type: "precedes", To: "T-1"},
	}
	tests := []struct {
		name        string
		judged      map[entity.RelationKey]bool
		wantUpdated int
	}{
		{name: "no restriction", judged: nil, wantUpdated: 1},
		{name: "all judged", judged: map[entity.RelationKey]bool{keys[0]: true, keys[1]: true}, wantUpdated: 1},
		{name: "one unjudged", judged: map[entity.RelationKey]bool{keys[0]: true}, wantUpdated: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := memstore.New()
			for _, id := range []string{"A-1", "B-1", "T-1"} {
				if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "task"}); err != nil {
					t.Fatal(err)
				}
			}
			for i, k := range keys {
				data := &store.RelationData{Properties: map[string]any{prop: 1.0 + float64(i)*1e-12}}
				if _, err := st.CreateRelation(ctx, k, data); err != nil {
					t.Fatal(err)
				}
			}
			updated, err := maybeRenumberSide(ctx, st, q, prop, tc.judged)
			if err != nil {
				t.Fatal(err)
			}
			if len(updated) != tc.wantUpdated {
				t.Fatalf("updated %d edges, want %d", len(updated), tc.wantUpdated)
			}
			if tc.wantUpdated == 0 {
				r, err := st.GetRelation(ctx, keys[1])
				if err != nil {
					t.Fatal(err)
				}
				if v, _ := FiniteOrder(r.Properties[prop]); v == 2 {
					t.Errorf("unjudged sibling was rewritten to %v", v)
				}
			}
		})
	}
}
