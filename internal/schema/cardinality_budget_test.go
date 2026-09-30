package schema_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// budgetMeta declares a bound on every direction, so every read the check
// can make is exercised.
func budgetMeta() *metamodel.Metamodel {
	one, five := 1, 5
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefixes: []string{"TKT-"}},
			"concept": {Label: "Concept", IDPrefixes: []string{"CON-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects": {
				From: []string{"ticket"}, To: []string{"concept"},
				MinOutgoing: &one, MaxOutgoing: &five, MinIncoming: &one, MaxIncoming: &five,
			},
		},
	}
}

// seedBudget stores n tickets and n concepts; even tickets affect the
// concept with the same number, so half of each side violates a min bound.
func seedBudget(t *testing.T, st store.Store, n int) {
	t.Helper()
	ctx := context.Background()
	for i := range n {
		tkt, con := fmt.Sprintf("TKT-%03d", i), fmt.Sprintf("CON-%03d", i)
		for _, e := range []*entity.Entity{{ID: tkt, Type: "ticket"}, {ID: con, Type: "concept"}} {
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Fatalf("seed %s: %v", e.ID, err)
			}
		}
		if i%2 == 0 {
			if _, err := st.CreateRelation(ctx, entity.RelationKey{From: tkt, Type: "affects", To: con}, nil); err != nil {
				t.Fatalf("seed relation: %v", err)
			}
		}
	}
}

// TestCheckCardinality_ReadBudget pins TKT-5LW875: the check reads edges
// once per relation and subjects once per type, so its store reads do not
// grow with the number of subjects. The per-subject CountRelations it
// replaced made one read per subject per bound.
func TestCheckCardinality_ReadBudget(t *testing.T) {
	reads := func(n int) (int, int) {
		counting := storetest.NewCounting(memstore.New())
		seedBudget(t, counting, n)
		counting.Reset()
		violations, err := schema.CheckCardinality(context.Background(), schema.Ungated(counting), budgetMeta(), nil)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		return counting.Reads(), len(violations)
	}

	small, smallViolations := reads(10)
	large, largeViolations := reads(50)
	if small != large {
		t.Errorf("reads grow with subjects: %d at 10, %d at 50", small, large)
	}
	// Half of each side violates its min bound: n/2 tickets and n/2 concepts.
	if smallViolations != 10 || largeViolations != 50 {
		t.Errorf("violations = %d at 10, %d at 50; want 10 and 50", smallViolations, largeViolations)
	}
}

// failingRelationReader fails every ListRelations call. Store must be
// non-nil: ListEntities delegates to it.
type failingRelationReader struct {
	store.Store
	err error
}

func (f failingRelationReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) {
		yield(nil, f.err)
	}
}

// TestCheckCardinality_EdgeReadErrorAborts pins that a failed edge read
// fails the check. Counting around it would report every subject as having
// zero edges, which invents a min violation for each one.
func TestCheckCardinality_EdgeReadErrorAborts(t *testing.T) {
	st := memstore.New()
	seedBudget(t, st, 2)
	readErr := errors.New("backend down")

	violations, err := schema.CheckCardinality(
		context.Background(), schema.Ungated(failingRelationReader{Store: st, err: readErr}), budgetMeta(), nil)
	if !errors.Is(err, readErr) {
		t.Fatalf("want the read error, got err=%v", err)
	}
	if len(violations) != 0 {
		t.Errorf("returned violations alongside a failed read: %+v", violations)
	}
}

// TestCheckCardinality_CountsOnlyYieldedEdges pins that the reader is the
// gate: an edge the reader does not yield is not counted, whatever the
// store holds. A gated reader relies on this to keep a hidden neighbor out
// of the count.
func TestCheckCardinality_CountsOnlyYieldedEdges(t *testing.T) {
	st := memstore.New()
	seedBudget(t, st, 2) // TKT-000 affects CON-000; TKT-001 affects nothing

	hidden := hideRelationsTo{Store: st, hidden: "CON-000"}
	violations, err := schema.CheckCardinality(context.Background(), schema.Ungated(hidden), budgetMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, v := range violations {
		if v.Constraint == "min_outgoing" {
			got[v.EntityID] = v.Actual
		}
	}
	if n, ok := got["TKT-000"]; !ok || n != 0 {
		t.Errorf("TKT-000 min_outgoing: want a violation with count 0, got %v (present=%v)", n, ok)
	}
}

// hideRelationsTo drops every edge whose head is hidden, as a gated reader
// drops an edge to an entity the caller may not read.
type hideRelationsTo struct {
	store.Store
	hidden string
}

func (h hideRelationsTo) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) {
		for rel, err := range h.Store.ListRelations(ctx, q) {
			if err == nil && rel.To == h.hidden {
				continue
			}
			if !yield(rel, err) {
				return
			}
		}
	}
}

// TestCheckCardinality_ScopeCountsBothDirections pins the scope pushdown: a
// scoped run asks only for edges touching a scoped id, and that must include
// an edge whose other end is out of scope, on either side.
func TestCheckCardinality_ScopeCountsBothDirections(t *testing.T) {
	st := memstore.New()
	seedBudget(t, st, 2) // TKT-000 affects CON-000; TKT-001 and CON-001 have none
	for _, tc := range []struct {
		name    string
		scope   string
		wantMin []string
	}{
		{name: "incoming edge from an out-of-scope tail", scope: "CON-000"},
		{name: "outgoing edge to an out-of-scope head", scope: "TKT-000"},
		{name: "no edge", scope: "CON-001", wantMin: []string{"CON-001"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := schema.CheckCardinality(context.Background(), schema.Ungated(st), budgetMeta(),
				map[string]bool{tc.scope: true})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, v := range violations {
				got = append(got, v.EntityID)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.wantMin) {
				t.Errorf("violations = %v, want %v", got, tc.wantMin)
			}
		})
	}
}

// nilRelationReader yields a nil relation, as a faulty reader might.
type nilRelationReader struct{ store.Store }

func (nilRelationReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) { yield(nil, nil) }
}

// TestCheckCardinality_NilEdgeAborts: a nil row fails the check instead of
// being skipped, which would lower a count and invent a min violation.
func TestCheckCardinality_NilEdgeAborts(t *testing.T) {
	st := memstore.New()
	seedBudget(t, st, 2)
	violations, err := schema.CheckCardinality(context.Background(), schema.Ungated(nilRelationReader{st}),
		budgetMeta(), nil)
	if err == nil || len(violations) != 0 {
		t.Fatalf("got %v, %v; want an error and no violations", violations, err)
	}
}
