package storetest

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// nestingListers is how many iterations run at once. It must exceed the
// connection pool a database backend's test factory builds (two for pgstore),
// so a backend that holds a connection across yield cannot finish.
const nestingListers = 4

// nestingTimeout bounds each case. A held connection shows up as a deadline,
// not as a hang.
const nestingTimeout = 10 * time.Second

// iterCase is one store iterator, reduced to the IDs it yields. nested is the
// per-row store call caller code makes; a method that takes a callback (the
// hidden-fields func) makes it there too.
type iterCase struct {
	name string
	ids  func(ctx context.Context, s store.Store, vs search.VisibleSearcher, nested func(string) error) iter.Seq2[string, error]
}

// RunIteratorNestingTests pins that a caller may make store calls inside the
// body of any store iteration (BUG-9TGOH1). Visibility redaction does exactly
// that for every row. A backend that holds a pooled connection across yield
// needs a second connection per concurrent iteration, and deadlocks once the
// iterations outnumber the pool. On a Tx view, the same backend would issue the
// nested statement on a connection whose result is still being read.
//
// vsf may be nil; the visible-search cases are then skipped.
func RunIteratorNestingTests(t *testing.T, f Factory, vsf VisibleSearchFactory) {
	t.Helper()
	for _, tc := range nestingCases() {
		t.Run(tc.name, func(t *testing.T) {
			s, vs := nestingStore(t, f, vsf, tc.name)
			runConcurrentNesting(t, s, vs, tc)
		})
	}
	// A Tx view runs every statement on one connection, so a nested call
	// fails at once if the iteration still holds it.
	for _, tc := range nestingCases() {
		if isSearchCase(tc.name) {
			continue
		}
		t.Run("InsideTx/"+tc.name, func(t *testing.T) {
			s := f(t)
			seedNesting(t, s)
			ctx, cancel := context.WithTimeout(t.Context(), nestingTimeout)
			defer cancel()
			require.NoError(t, s.Tx(ctx, func(tx store.Store) error {
				nested := func(id string) error { return nestedGet(ctx, tx, id) }
				n := 0
				for id, err := range tc.ids(ctx, tx, nil, nested) {
					if err == nil {
						err = nested(id)
					}
					if err != nil {
						return fmt.Errorf("nested call inside Tx iteration: %w", err)
					}
					n++
				}
				if n == 0 {
					return errors.New("Tx iteration saw no rows")
				}
				return nil
			}))
		})
	}
}

func isSearchCase(name string) bool {
	return name == "SearchVisible" || name == "SearchVisibleFields"
}

func nestingCases() []iterCase {
	scope := map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
	gq := store.GraphQuery{EntityType: "feature"}
	byEntity := func(e *entity.Entity) string { return e.ID }
	byHeader := func(h store.EntityHeader) string { return h.ID }
	byHit := func(h search.Hit) string { return h.ID }
	return []iterCase{
		{"ListEntities", func(ctx context.Context, s store.Store, _ search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			return idSeq(s.ListEntities(ctx, store.EntityQuery{Type: "feature"}), byEntity)
		}},
		{"ListEntityHeaders", func(ctx context.Context, s store.Store, _ search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			return idSeq(store.ListEntityHeaders(ctx, s, store.EntityQuery{Type: "feature"}), byHeader)
		}},
		{"ListRelations", func(ctx context.Context, s store.Store, _ search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			return idSeq(s.ListRelations(ctx, store.RelationQuery{Type: "nest"}),
				func(r *entity.Relation) string { return r.From })
		}},
		{"GraphQuery", func(ctx context.Context, s store.Store, _ search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			return idSeq(s.GraphQuery(ctx, gq), byEntity)
		}},
		{"GraphQueryLimit", func(ctx context.Context, s store.Store, _ search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			limited := gq
			limited.Limit = 4
			return idSeq(s.GraphQuery(ctx, limited), byEntity)
		}},
		{"GraphQueryHeaders", func(ctx context.Context, s store.Store, _ search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			return idSeq(store.GraphQueryHeaders(ctx, s, gq), byHeader)
		}},
		{"SearchVisible", func(ctx context.Context, _ store.Store, vs search.VisibleSearcher, _ func(string) error) iter.Seq2[string, error] {
			return idSeq(vs.SearchVisible(ctx, search.Query{Text: "nesting"}, scope), byHit)
		}},
		{"SearchVisibleFields", func(ctx context.Context, _ store.Store, vs search.VisibleSearcher, nested func(string) error) iter.Seq2[string, error] {
			fvs, ok := vs.(search.FieldVisibleSearcher)
			if !ok { // unreachable: nestingStore skips such searchers
				return func(yield func(string, error) bool) { yield("", errors.New("no field-level filter")) }
			}
			// The hidden-fields callback is caller code too: it computes
			// field verdicts, which make store calls of their own.
			hidden := func(_ context.Context, h search.Hit, _ *entity.Entity) (map[string]struct{}, error) {
				return nil, nested(h.ID)
			}
			return idSeq(fvs.SearchVisibleFields(ctx, search.Query{Text: "nesting"}, scope, hidden), byHit)
		}},
	}
}

// nestingStore builds and seeds the store a case runs against. Search cases
// need the visible searcher and skip without one.
func nestingStore(
	t *testing.T, f Factory, vsf VisibleSearchFactory, name string,
) (store.Store, search.VisibleSearcher) {
	t.Helper()
	var (
		s  store.Store
		vs search.VisibleSearcher
	)
	switch {
	case isSearchCase(name):
		if vsf == nil {
			t.Skip("no VisibleSearchFactory")
		}
		s, _, vs = vsf(t)
		if _, ok := vs.(search.FieldVisibleSearcher); !ok && name == "SearchVisibleFields" {
			t.Skip("visible searcher has no field-level filter")
		}
	default:
		s = f(t)
	}
	seedNesting(t, s)
	return s, vs
}

func seedNesting(t *testing.T, s store.Store) {
	t.Helper()
	ctx := t.Context()
	for i := range 5 {
		e := entity.New(fmt.Sprintf("NEST-%d", i), "feature")
		e.SetString("title", "nesting probe")
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	for i := range 4 {
		_, err := s.CreateRelation(ctx, fmt.Sprintf("NEST-%d", i), "nest", fmt.Sprintf("NEST-%d", i+1), nil)
		require.NoError(t, err)
	}
}

// runConcurrentNesting runs nestingListers iterations of tc at once, each
// making a nested store call per row, and requires every one to finish and
// see rows.
func runConcurrentNesting(t *testing.T, s store.Store, vs search.VisibleSearcher, tc iterCase) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), nestingTimeout)
	defer cancel()
	nested := func(id string) error { return nestedGet(ctx, s, id) }

	errs := make([]error, nestingListers)
	counts := make([]int, nestingListers)
	var wg sync.WaitGroup
	for w := range nestingListers {
		wg.Go(func() {
			for id, err := range tc.ids(ctx, s, vs, nested) {
				if err == nil {
					err = nested(id)
				}
				if err != nil {
					errs[w] = err
					return
				}
				counts[w]++
			}
		})
	}
	wg.Wait()
	for w := range nestingListers {
		require.NoError(t, errs[w], "iteration %d (connection held across yield?)", w)
		require.Positive(t, counts[w], "iteration %d saw no rows", w)
	}
}

// nestedGet is the per-row store call redaction makes: an ACL edge check.
func nestedGet(ctx context.Context, s store.Store, id string) error {
	_, err := s.GetRelation(ctx, id, "nest", "NO-SUCH-TARGET")
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}

func idSeq[T any](seq iter.Seq2[T, error], id func(T) string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for v, err := range seq {
			if err != nil {
				yield("", err)
				return
			}
			if !yield(id(v), nil) {
				return
			}
		}
	}
}
