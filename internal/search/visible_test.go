package search_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// TestVisibleConformance_Bleve runs the VisibleSearcher conformance
// suite over the generic wrapper with the bleve backend — the
// combination the default (fsstore) build ships. The linear-backend
// runs live in the memstore/fsstore conformance tests; pgstore's
// native implementation has its own DB-gated run.
func TestVisibleConformance_Bleve(t *testing.T) {
	storetest.RunVisibleSearchTests(t, func(t *testing.T) (store.Store, search.Searcher, search.VisibleSearcher) {
		t.Helper()
		idx, err := bleveindex.NewMem()
		require.NoError(t, err)
		t.Cleanup(func() { _ = idx.Close() })
		s := memstore.New(memstore.WithObserver(idx))
		searcher := search.New(s, idx)
		v, err := search.NewVisible(searcher, s)
		require.NoError(t, err)
		return s, searcher, v
	})
}

func TestNewVisible_RejectsNil(t *testing.T) {
	s := memstore.New()
	searcher := search.New(s, search.NewLinearSearch())

	if _, err := search.NewVisible(nil, s); err == nil {
		t.Error("nil inner Searcher accepted")
	}
	if _, err := search.NewVisible(searcher, nil); err == nil {
		t.Error("nil GraphQueryer accepted")
	}
}

// nonProvenanceSearcher wraps a Searcher WITHOUT forwarding match provenance —
// standing in for any decorator (metrics, caching, tracing) that a future
// refactor might slip between the Service and NewVisible. It must NOT satisfy
// the unexported fieldMatchProvenance interface.
type nonProvenanceSearcher struct{ inner search.Searcher }

func (n nonProvenanceSearcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return n.inner.Search(ctx, q)
}

// TestSearchVisible_FieldScopeFailsClosedWithoutProvenance: when the wired
// searcher cannot report match provenance, a search over a field-gated type
// must FAIL CLOSED (ErrScope), never return unchecked hits. A wrapped Service
// loses the provenance type assertion; this proves that path errors instead
// of leaking.
func TestSearchVisible_FieldScopeFailsClosedWithoutProvenance(t *testing.T) {
	s := memstore.New()
	e := entity.New("TKT-1", "ticket")
	e.SetString("title", "alpha rocket")
	e.SetString("code", "zeta777")
	if err := s.CreateEntity(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	ls := search.NewLinearSearch()
	_ = ls.EntityPut(e)

	wrapped := nonProvenanceSearcher{inner: search.New(s, ls)}
	v, err := search.NewVisible(wrapped, s)
	if err != nil {
		t.Fatal(err)
	}
	scope := map[string]search.TypeScope{"ticket": {AllowAll: true, Fields: &search.FieldScope{Gated: []string{"code"}}}}

	var streamErr error
	hits := 0
	for _, iterErr := range v.SearchVisible(
		context.Background(), search.Query{Text: "zeta777", World: store.TrivialScope()}, scope) {
		if iterErr != nil {
			streamErr = iterErr
			break
		}
		hits++
	}
	if hits != 0 {
		t.Errorf("leak: %d hits returned without provenance; want fail-closed", hits)
	}
	if !errors.Is(streamErr, search.ErrScope) {
		t.Errorf("want ErrScope on missing provenance, got %v", streamErr)
	}
}

// TestSearchVisible_FieldScopeReadBudget pins the field check's cost: the
// reads it adds on top of the same search without a field scope are the same
// for 10 matching rows as for 50, and with a shrunken page size the same per
// page. A per-hit entity read or a per-hit grant query would grow with the
// rows. (The inner Service's own per-hit read is
// the ungated search's cost and is not measured here.)
func TestSearchVisible_FieldScopeReadBudget(t *testing.T) {
	fields := &search.FieldScope{
		Gated: []string{"code", "status", "title"},
		Grants: []search.FieldGrant{
			{Fields: []string{"status"}},
			{Fields: []string{"code"}, When: &store.GraphQuery{
				Props: []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
			}},
		},
	}
	page := 0 // 0 keeps the production page size
	added := func(rows int) map[string]int {
		t.Helper()
		ms := memstore.New()
		ls := search.NewLinearSearch()
		for i := range rows {
			e := entity.New(fmt.Sprintf("TKT-%03d", i), "ticket")
			e.SetString("title", "row")
			e.SetString("code", "budget code")
			e.SetString("status", []string{"open", "done"}[i%2])
			if err := ms.CreateEntity(t.Context(), e); err != nil {
				t.Fatal(err)
			}
			_ = ls.EntityPut(e)
		}
		counting := storetest.NewCounting(ms)
		v, err := search.NewVisible(search.New(counting, ls), counting)
		if err != nil {
			t.Fatal(err)
		}
		if page > 0 {
			search.SetFieldCheckPageForTest(v, page)
		}
		measure := func(fs *search.FieldScope, want int) map[string]int {
			counting.Reset()
			scope := map[string]search.TypeScope{"ticket": {AllowAll: true, Fields: fs}}
			n := 0
			q := search.Query{Text: "budget", World: store.TrivialScope()}
			for _, err := range v.SearchVisible(t.Context(), q, scope) {
				if err != nil {
					t.Fatal(err)
				}
				n++
			}
			if n != want {
				t.Fatalf("%d rows: got %d hits, want %d", rows, n, want)
			}
			return counting.Calls()
		}
		base := measure(nil, rows)
		gated := measure(fields, rows/2) // only open rows show code
		for k, n := range base {
			gated[k] -= n
			if gated[k] == 0 {
				delete(gated, k)
			}
		}
		return gated
	}
	small, large := added(10), added(50)
	if !maps.Equal(small, large) {
		t.Errorf("field check reads grow with rows: 10 rows %v, 50 rows %v", small, large)
	}
	if len(small) == 0 {
		t.Error("field check made no reads; the budget measures nothing")
	}

	// With pages of 5 hits, 10 rows take 2 pages and 50 rows take 10. The
	// reads per page must be the same, or a page's cost grows with its rows.
	page = 5
	two, ten := added(10), added(50)
	if len(two) == 0 {
		t.Fatal("paged field check made no reads; the budget measures nothing")
	}
	for k, n := range two {
		if n%2 != 0 || ten[k]%10 != 0 || n/2 != ten[k]/10 {
			t.Errorf("%s: %d reads over 2 pages, %d over 10 pages; want the same per page", k, n, ten[k])
		}
	}
	for k := range ten {
		if _, ok := two[k]; !ok {
			t.Errorf("%s: read only at 50 rows", k)
		}
	}
}

// TestSearchVisible_BleveHiddenOnlyMatchDropped: bleve matches the word
// "foo,bar" against the hidden code "foobar" (one edit). The visible title
// "foo visible" holds the analyzer token "foo" but not the word, so it must
// not count as the match.
func TestSearchVisible_BleveHiddenOnlyMatchDropped(t *testing.T) {
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })
	s := memstore.New(memstore.WithObserver(idx))
	e := entity.New("TKT-1", "ticket")
	e.SetString("title", "foo visible")
	e.SetString("code", "foobar")
	require.NoError(t, s.CreateEntity(t.Context(), e))
	svc := search.New(s, idx)
	v, err := search.NewVisible(svc, s)
	require.NoError(t, err)

	q := search.Query{Text: "foo,bar", World: store.TrivialScope()}
	ungated := 0
	for _, err := range svc.Search(t.Context(), q) {
		require.NoError(t, err)
		ungated++
	}
	require.Equal(t, 1, ungated, "fixture: bleve must match the hidden code")

	scope := map[string]search.TypeScope{"ticket": {AllowAll: true, Fields: &search.FieldScope{
		Gated: []string{"code", "title"}, Grants: []search.FieldGrant{{Fields: []string{"title"}}},
	}}}
	for h, err := range v.SearchVisible(t.Context(), q, scope) {
		require.NoError(t, err)
		t.Errorf("hit %s returned; its only match is the hidden code", h.ID)
	}
}
