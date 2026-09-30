package cli

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestRenumber_FacedTails pins that `rela renumber` plans each tail of one
// triple as its own sibling and writes to that tail (TKT-KQXVF7). Before, the
// plan keyed siblings by triple, and the write carried no face, so it went
// to the default tail, which a faced source does not have.
func TestRenumber_FacedTails(t *testing.T) {
	captureOut(t)
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
	draft := entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "CTL-1"}
	published := entity.RelationKey{From: "POL-1", FromFace: "published", Type: "cites", To: "CTL-1"}
	for k, v := range map[entity.RelationKey]float64{draft: 5, published: 9} {
		if _, err := st.CreateRelation(ctx, k, &store.RelationData{Properties: map[string]any{prop: v}}); err != nil {
			t.Fatal(err)
		}
	}
	meta := &metamodel.Metamodel{Relations: map[string]metamodel.RelationDef{
		"cites": {Orderable: metamodel.OrderableIncoming},
	}}

	w := &recordingRelationWriter{}
	svc := &writeServices{readServices: readServices{Store: st, Meta: meta}, EntityManager: w}
	if err := (&RenumberCmd{}).Run(ctx, svc); err != nil {
		t.Fatalf("renumber: %v", err)
	}
	slices.SortFunc(w.updates, func(a, b entity.RelationKey) int {
		return strings.Compare(a.String(), b.String())
	})
	want := []entity.RelationKey{draft, published}
	if !slices.Equal(w.updates, want) {
		t.Errorf("updates = %v, want %v", w.updates, want)
	}
}
