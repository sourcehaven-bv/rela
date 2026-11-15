package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// seedInterleavedEdges gives TKT-0 an edge to n tickets and n secrets, so
// hidden edges sit between visible ones in key order.
func seedInterleavedEdges(t *testing.T, n int) store.Store {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	add := func(e *entity.Entity) {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	add(&entity.Entity{ID: "TKT-0", Type: "ticket"})
	for i := range n {
		for _, e := range []*entity.Entity{
			{ID: fmt.Sprintf("TKT-%d-a", i), Type: "ticket"},
			{ID: fmt.Sprintf("TKT-%d-b", i), Type: "secret"},
		} {
			add(e)
			if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-0", Type: "relates", To: e.ID}, nil); err != nil {
				t.Fatalf("seed edge: %v", err)
			}
		}
	}
	return st
}

// TestScriptReader_ListRelationsPage pins the gated relation page: hidden
// edges neither show nor shorten a page, and a cursor always follows a row
// the caller may read, so it never encodes a hidden key.
func TestScriptReader_ListRelationsPage(t *testing.T) {
	const n = 7
	sr := newTicketOnlyScriptReader(t, seedInterleavedEdges(t, n))
	ctx := context.Background()

	var got []string
	q := store.RelationQuery{From: "TKT-0", Limit: 2}
	for pages := 0; ; pages++ {
		if pages > n {
			t.Fatal("paging did not end")
		}
		page, err := sr.ListRelationsPage(ctx, q)
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		for _, r := range page.Items {
			got = append(got, r.To)
		}
		if page.NextCursor == "" {
			break
		}
		if len(page.Items) != q.Limit {
			t.Fatalf("page with a cursor has %d rows, want %d", len(page.Items), q.Limit)
		}
		key, err := storeutil.DecodeCursor(page.NextCursor)
		if err != nil {
			t.Fatalf("decode cursor: %v", err)
		}
		if last := page.Items[len(page.Items)-1]; key != last.Key() {
			t.Fatalf("cursor follows %q, want the last served row %q", key, last.Key())
		}
		q.Cursor = page.NextCursor
	}

	var want []string
	for i := range n {
		want = append(want, fmt.Sprintf("TKT-%d-a", i))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("relations = %v, want %v", got, want)
	}
}

func TestScriptReader_ListRelationsPageRejectsNoLimit(t *testing.T) {
	sr := newTicketOnlyScriptReader(t, seedScriptWorld(t))
	if _, err := sr.ListRelationsPage(context.Background(), store.RelationQuery{From: "TKT-1"}); err == nil {
		t.Error("a page without a limit was served")
	}
	ur := visibility.Unrestricted(seedScriptWorld(t))
	if _, err := ur.ListRelationsPage(context.Background(), store.RelationQuery{}); err == nil {
		t.Error("unrestricted relation page without a limit was served")
	}
	if _, err := ur.ListEntitiesPage(context.Background(), store.EntityQuery{}); err == nil {
		t.Error("unrestricted entity page without a limit was served")
	}
}

// stuckStore hands back an empty page with a cursor, which would loop.
type stuckStore struct{ store.Store }

func (stuckStore) ListRelationsPage(context.Context, store.RelationQuery) (store.Page[*entity.Relation], error) {
	return store.Page[*entity.Relation]{NextCursor: storeutil.EncodeCursor("x")}, nil
}

// countingPages counts the raw relation page reads behind a ScriptReader.
type countingPages struct {
	store.Store
	calls int
}

func (c *countingPages) ListRelationsPage(
	ctx context.Context, q store.RelationQuery,
) (store.Page[*entity.Relation], error) {
	c.calls++
	return c.Store.ListRelationsPage(ctx, q)
}

// TestScriptReader_ListRelationsPageHiddenEdgesCostChunks pins the refill
// size: hidden edges before the first visible one cost one raw read per
// chunk of rows, not one per edge.
func TestScriptReader_ListRelationsPageHiddenEdgesCostChunks(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	seed := func(id, typ string) {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: typ}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-0", Type: "relates", To: id}, nil); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "TKT-0", Type: "ticket"}); err != nil {
		t.Fatal(err)
	}
	const hidden = 2000
	for i := range hidden {
		seed(fmt.Sprintf("SEC-%04d", i), "secret")
	}
	seed("TKT-9", "ticket") // sorts after every SEC- edge
	seed("TKT-A", "ticket")

	spy := &countingPages{Store: st}
	sr := newTicketOnlyScriptReader(t, spy)
	page, err := sr.ListRelationsPage(ctx, store.RelationQuery{From: "TKT-0", Limit: 1})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].To != "TKT-9" {
		t.Fatalf("page = %v, want only TKT-9", page.Items)
	}
	// One exact first read, hidden/512 rounded up refills, one cursor re-read.
	if limit := 2 + (hidden+511)/512; spy.calls > limit {
		t.Errorf("raw page reads = %d, want at most %d", spy.calls, limit)
	}

	next, err := sr.ListRelationsPage(ctx, store.RelationQuery{From: "TKT-0", Limit: 1, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("next page: %v", err)
	}
	if len(next.Items) != 1 || next.Items[0].To != "TKT-A" {
		t.Errorf("next page = %v, want TKT-A: the cursor must resume right after the row served", next.Items)
	}
}

// faultGate fails every row gate call.
type faultGate struct{}

func (faultGate) PermitsRead(context.Context, string, string) (bool, error) {
	return false, errors.New("gate down")
}

func (faultGate) ReadableFacesMany(context.Context, string, []string) (acl.FaceVerdicts, error) {
	return acl.FaceVerdicts{}, errors.New("gate down")
}

// TestScriptReader_ListRelationsPageReturnsGateFaults pins that a gate fault
// fails the page instead of thinning it to a short list read as complete.
func TestScriptReader_ListRelationsPageReturnsGateFaults(t *testing.T) {
	st := seedScriptWorld(t)
	reader, err := visibility.NewPolicyReader(faultGate{}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(reader, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	sr = sr.WithWorld(visibility.WorldOf(store.TrivialScope()))
	if _, err := sr.ListRelationsPage(context.Background(), store.RelationQuery{From: "TKT-1", Limit: 5}); err == nil {
		t.Error("a gate fault was served as a page")
	}
}

func TestScriptReader_ListRelationsPageRefusesStuckCursor(t *testing.T) {
	sr := newTicketOnlyScriptReader(t, stuckStore{seedScriptWorld(t)})
	if _, err := sr.ListRelationsPage(context.Background(), store.RelationQuery{Limit: 2}); err == nil {
		t.Error("a store that does not advance was paged without an error")
	}
}
