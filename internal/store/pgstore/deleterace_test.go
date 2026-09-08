package pgstore_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace pins the family
// delete's row lock. Before it, DeleteEntity scanned the family on a READ
// COMMITTED snapshot while CreateEntity's FOR SHARE probe let a state create
// commit in the window, so PAGE-k vanished and PAGE-k@draft survived headless.
//
// With the bare row locked FOR UPDATE the two serialize: the create either
// commits first and is swept with the family, or waits and then finds no
// family (refused as headless). Either way NO row of the id remains. The
// interleaving is driven by real concurrency rather than a scripted replay,
// so the case is repeated; a single leaked row fails it.
func TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	const rounds = 25
	for i := range rounds {
		id := "PAGE-" + string(rune('A'+i%26)) + string(rune('A'+i/26))
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "page")))

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			draft := entity.New(id, "page")
			draft.Face = "draft"
			_ = s.CreateEntity(ctx, draft) // may succeed or be refused as headless
		}()
		go func() {
			defer wg.Done()
			_, _ = s.DeleteEntity(ctx, id, false)
		}()
		wg.Wait()

		// Whatever the order, the family must be gone in full: a surviving
		// draft with no bare row is the corruption this lock prevents.
		var left []string
		for e, err := range s.ListEntities(ctx, store.EntityQuery{IDs: []string{id}, AllStates: true}) {
			require.NoError(t, err)
			left = append(left, e.ID+"@"+e.Face.String())
		}
		require.Empty(t, left, "round %d: rows survived a delete that raced a state create", i)
	}
}
