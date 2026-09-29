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

	updated, err := maybeRenumberSide(ctx, st, store.RelationQuery{To: "CTL-1", Type: "cites"}, prop)
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
