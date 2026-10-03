package datamigration

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// seedTailedEdge seeds PAGE-1 with a draft face and one `cites` edge tailed
// on it, carrying a `note` property.
func seedTailedEdge(t *testing.T) (store.Store, entity.RelationKey) {
	t.Helper()
	st := memstore.New()
	ctx := t.Context()
	for _, e := range []*entity.Entity{
		{ID: "PAGE-1", Type: "page", Face: "draft"},
		{ID: "SPEC-1", Type: "spec"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	k := entity.RelationKey{From: "PAGE-1", FromFace: "draft", Type: "cites", To: "SPEC-1"}
	data := &store.RelationData{Properties: map[string]any{"note": "x"}, Content: "body"}
	if _, err := st.CreateRelation(ctx, k, data); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	return st, k
}

func onlyRelation(t *testing.T, st store.Store) *entity.Relation {
	t.Helper()
	var got []*entity.Relation
	for r, err := range st.ListRelations(t.Context(), store.RelationQuery{}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if len(got) != 1 {
		t.Fatalf("relations = %v, want exactly one", got)
	}
	return got[0]
}

// TestRelationSteps_KeepTheTail pins that the relation steps address an edge
// by its full key (TKT-KQXVF7). Before, each step dropped the tail: a rename
// minted a default-tail copy and left the draft edge behind, and a drop or a
// property GC missed the draft edge.
func TestRelationSteps_KeepTheTail(t *testing.T) {
	t.Run("rename_relation_type", func(t *testing.T) {
		st, k := seedTailedEdge(t)
		x := &Exec{Store: st, Apply: true}
		if _, err := (&renameRelationTypeStep{From: "cites", To: "refs"}).Run(t.Context(), x); err != nil {
			t.Fatal(err)
		}
		got := onlyRelation(t, st)
		want := k
		want.Type = "refs"
		if got.Identity() != want {
			t.Errorf("renamed edge = %s, want %s", got.Identity(), want)
		}
		if got.Content != "body" || got.Properties["note"] != "x" {
			t.Errorf("renamed edge lost its data: %+v", got)
		}
	})

	t.Run("drop_relations", func(t *testing.T) {
		st, _ := seedTailedEdge(t)
		x := &Exec{Store: st, Apply: true}
		if _, err := (&dropRelationsStep{Type: "cites"}).Run(t.Context(), x); err != nil {
			t.Fatal(err)
		}
		n, err := st.CountRelations(t.Context(), store.RelationQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%d relations survived the drop", n)
		}
	})

	t.Run("drop_relation_property", func(t *testing.T) {
		st, k := seedTailedEdge(t)
		x := &Exec{Store: st, Apply: true}
		if _, err := dropRelationProperty(t.Context(), x, "cites", "note"); err != nil {
			t.Fatal(err)
		}
		got := onlyRelation(t, st)
		if got.Identity() != k {
			t.Errorf("edge = %s, want %s", got.Identity(), k)
		}
		if _, has := got.Properties["note"]; has {
			t.Errorf("property survived the GC: %+v", got.Properties)
		}
	})
}
