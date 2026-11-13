package storetest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// recordingTB captures Errorf calls so a test can assert a helper fails.
type recordingTB struct {
	testing.TB
	errors []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// docStore returns a counting store holding n docs, counters reset.
func docStore(ctx context.Context, t *testing.T, n int) *storetest.Counting {
	t.Helper()
	c := storetest.NewCounting(memstore.New())
	for i := range n {
		seedDoc(ctx, t, c, fmt.Sprintf("DOC-%04d", i))
	}
	c.Reset()
	return c
}

// TestAssertScaleFree_FullScanFails pins the reason rows are counted: one
// call that reads every row costs the same CALLS at both sizes, so only the
// rows tell it from a paged read.
func TestAssertScaleFree_FullScanFails(t *testing.T) {
	ctx := context.Background()
	rec := &recordingTB{TB: t}
	storetest.AssertScaleFree(rec, func(n int) (storetest.Cost, string) {
		c := docStore(ctx, t, n)
		for _, err := range c.ListEntityHeaders(ctx, store.EntityQuery{Type: "doc", Faces: store.AllFaces()}) {
			if err != nil {
				t.Fatal(err)
			}
		}
		return c.Cost(), c.String()
	})
	if len(rec.errors) != 1 || !strings.Contains(rec.errors[0], "rows read grow") {
		t.Fatalf("a full scan must fail on rows only, got %q", rec.errors)
	}
}

// TestAssertScaleFree_PagedReadPasses is the control: a page of a fixed
// size reads the same rows however large the store is.
func TestAssertScaleFree_PagedReadPasses(t *testing.T) {
	ctx := context.Background()
	got := storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
		c := docStore(ctx, t, n)
		if _, err := c.ListEntitiesPage(ctx, store.EntityQuery{Type: "doc", Faces: store.AllFaces(), Limit: 20}); err != nil {
			t.Fatal(err)
		}
		return c.Cost(), c.String()
	})
	if want := (storetest.Cost{Calls: 1, Rows: 20}); got != want {
		t.Errorf("cost = %v, want %v", got, want)
	}
}

// TestCounting_Rows pins what counts as a row: what an iterator yields, a
// page carries or a point read finds; never a count.
func TestCounting_Rows(t *testing.T) {
	ctx := context.Background()
	c := docStore(ctx, t, 5)
	if _, err := c.CreateRelation(ctx, entity.RelationKey{From: "DOC-0000", Type: "refs", To: "DOC-0001"}, nil); err != nil {
		t.Fatal(err)
	}
	c.Reset()

	if _, err := c.GetEntity(ctx, entity.Ref{ID: "DOC-0000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetEntity(ctx, entity.Ref{ID: "missing"}); err == nil {
		t.Fatal("missing entity read without error")
	}
	if _, err := c.CountEntities(ctx, store.EntityQuery{Type: "doc", Faces: store.AllFaces()}); err != nil {
		t.Fatal(err)
	}
	// Stopping after two rows charges two, not five.
	seen := 0
	for _, err := range c.ListEntities(ctx, store.EntityQuery{Type: "doc", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		if seen++; seen == 2 {
			break
		}
	}
	for _, err := range c.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			t.Fatal(err)
		}
	}
	// A read inside a transaction counts on the same counter.
	if err := c.Tx(ctx, func(view store.Store) error {
		_, err := view.GetEntity(ctx, entity.Ref{ID: "DOC-0001"})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if got, want := c.Cost(), (storetest.Cost{Calls: 6, Rows: 5}); got != want {
		t.Errorf("cost = %v, want %v (%s)", got, want, c)
	}
	if got, want := c.String(), "CountEntities=1/0 GetEntity=3/2 ListEntities=1/2 ListRelations=1/1"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := c.Cost().Sub(storetest.Cost{Calls: 1, Rows: 2}); got != (storetest.Cost{Calls: 5, Rows: 3}) {
		t.Errorf("Sub = %v, want 5 calls, 3 rows", got)
	}
}
