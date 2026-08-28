package pgstore_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestListOrderIsByteWise guards that pgstore lists entities in Go byte order
// (matching fsstore/memstore), not the database's locale collation. The schema
// declares key columns COLLATE "C" for exactly this reason; without it,
// en_US.UTF-8 would order mixed-case/punctuated IDs differently from the
// in-memory backends and corrupt the keyset pagination cursor under a
// nondeterministic collation (regression: RR-8M34K / cranky review #1).
//
// The shared conformance suite can't catch this — its IDs are uniform
// zero-padded same-case (T-%03d), which sort identically under both orderings.
func TestListOrderIsByteWise(t *testing.T) {
	s := factory(t)
	ctx := context.Background()

	// IDs whose byte order differs from en_US.UTF-8 collation order. Under
	// en_US.UTF-8, 'a-9' sorts among the 'A-' ids; under byte order (C / Go),
	// uppercase precedes lowercase so 'a-9' sorts last.
	//
	// The lowercase id is deliberately NOT a case-variant of any other id
	// here: entity IDs are one identity per case-folding (BUG-3RCWNS), so a
	// fixture containing both 'a-2' and 'A-2' would now be rejected as a
	// conflict and could never be created. Mixed CASE across DISTINCT ids is
	// what this test needs, and that is still legal.
	ids := []string{"A-10", "a-9", "B-1", "A-2"}
	for _, id := range ids {
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "ticket")))
	}

	// Expected = Go's own byte-wise sort, the canonical cross-backend order.
	want := append([]string(nil), ids...)
	sort.Strings(want)

	var got []string
	for e, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		require.NoError(t, err)
		got = append(got, e.ID)
	}
	require.Equal(t, want, got, "ListEntities must return Go byte order, not locale collation")

	// The keyset paginator must agree with that order: paging 1-at-a-time must
	// reproduce the same sequence with no drops or duplicates.
	var paged []string
	cursor := ""
	for {
		page, err := s.ListEntitiesPage(ctx, store.EntityQuery{Limit: 1, Cursor: cursor})
		require.NoError(t, err)
		for _, e := range page.Items {
			paged = append(paged, e.ID)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	require.Equal(t, want, paged, "keyset pagination must match the byte-ordered full listing")
}
