package visibility_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// seedFacedPolicies stores n policies, each with a draft and a published
// face, and returns their ids.
func seedFacedPolicies(t *testing.T, st store.Store, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := range n {
		id := fmt.Sprintf("POL-%03d", i)
		ids = append(ids, id)
		for _, f := range []entity.Face{"draft", "published"} {
			if err := st.CreateEntity(context.Background(), &entity.Entity{ID: id, Type: "policy", Face: f}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return ids
}

// TestResolver_ResolveIDsBudget: the bare-id batch costs one header query
// and one per-face gate round, at 10 ids and at 50.
func TestResolver_ResolveIDsBudget(t *testing.T) {
	world := visibility.WorldOf(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
	}))
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			base := memstore.New()
			ids := seedFacedPolicies(t, base, n)
			st := storetest.NewCounting(base)
			gate := &countingGate{}
			r := mustResolver(t, gate, visibility.NopRedactor{}, st)
			got := r.ResolveIDs(context.Background(), world, ids)
			if len(got) != n {
				t.Fatalf("got %d ids, want %d", len(got), n)
			}
			for id, h := range got {
				if h.Face != "draft" {
					t.Fatalf("%s served at %q, want the world prime draft", id, h.Face)
				}
			}
			if st.Reads() != 1 || st.Calls()["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
			if gate.many != 1 || gate.single != 0 {
				t.Errorf("gate calls = %d many, %d single; want 1 and 0", gate.many, gate.single)
			}
		})
	}
}

// TestPolicyReader_FilterBudget: Filter asks the per-face gate once per
// distinct type and reads nothing from the store, at 10 rows and at 50.
func TestPolicyReader_FilterBudget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			base := memstore.New()
			seedFacedPolicies(t, base, n)
			st := storetest.NewCounting(base)
			gate := &countingGate{}
			pr, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			var rows []*entity.Entity
			for e, err := range base.ListEntities(context.Background(), store.EntityQuery{Faces: store.AllFaces()}) {
				if err != nil {
					t.Fatal(err)
				}
				rows = append(rows, e)
			}
			rows = append(rows, &entity.Entity{ID: "N-1", Type: "note"})
			st.Reset()
			if got := pr.Filter(context.Background(), rows); len(got) != len(rows) {
				t.Fatalf("got %d rows, want %d", len(got), len(rows))
			}
			if st.Reads() != 0 {
				t.Errorf("reads = %s, want none", st)
			}
			if gate.many != 2 || gate.single != 0 {
				t.Errorf("gate calls = %d many, %d single; want one per type (2) and 0", gate.many, gate.single)
			}
		})
	}
}
