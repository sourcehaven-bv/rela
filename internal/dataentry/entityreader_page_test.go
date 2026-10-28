package dataentry

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestPageRelationsDir pins how the batched edge read splits edges per row:
// a direction fills only its own slices, an edge between two rows lands in
// both, and rows that share an id each receive that id's edges.
func TestPageRelationsDir(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, id := range []string{"A", "B", "C"} {
		if err := st.CreateEntity(ctx, entity.New(id, "t")); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "A", Type: "r", To: "B"},
		{From: "C", Type: "r", To: "A"},
	} {
		if _, err := st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Two rows share id A, as two faces of one entity would.
	rows := []*entity.Entity{{ID: "A"}, {ID: "B"}, {ID: "A", Face: "f"}}
	keys := func(rels []*entity.Relation) []string {
		out := make([]string, 0, len(rels))
		for _, r := range rels {
			out = append(out, r.From+">"+r.To)
		}
		return out
	}

	tests := []struct {
		name    string
		dir     store.Direction
		wantOut [][]string
		wantIn  [][]string
	}{
		{"both", store.DirectionBoth,
			[][]string{{"A>B"}, {}, {"A>B"}}, [][]string{{"C>A"}, {"A>B"}, {"C>A"}}},
		{"outgoing", store.DirectionOutgoing,
			[][]string{{"A>B"}, {}, {"A>B"}}, [][]string{{}, {}, {}}},
		{"incoming", store.DirectionIncoming,
			[][]string{{}, {}, {}}, [][]string{{"C>A"}, {"A>B"}, {"C>A"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, in, err := entityReader{store: st}.pageRelationsDir(ctx, rows, tc.dir)
			if err != nil {
				t.Fatal(err)
			}
			for i := range rows {
				if got := keys(out[i]); !slices.Equal(got, tc.wantOut[i]) {
					t.Errorf("outgoing[%d] = %v, want %v", i, got, tc.wantOut[i])
				}
				if got := keys(in[i]); !slices.Equal(got, tc.wantIn[i]) {
					t.Errorf("incoming[%d] = %v, want %v", i, got, tc.wantIn[i])
				}
			}
		})
	}
}
