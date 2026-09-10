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

// TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace pinned the family
// delete against a racing state create. Its assertion — that NO row of the id
// survives — held because of the HEADLESS-STATE rule: a create landing after
// the delete found no zero-coordinate row and was refused.
//
// BUG-HC6I2T removed that rule (no face is privileged by storage), so the
// refusal is gone. The interleaving half is now covered by lockFamily's
// advisory lock, which is what this test was really protecting: no row can
// land between the delete's family scan and its sweep.
//
// What is left is a semantic question, not a locking one — whether a create
// that wins the lock and finds the family absent is a NEW entity reusing a
// freed id (the current answer) or should be refused. Skipped rather than
// rewritten, because the assertion depends on that decision. BUG-22XSH3
// carries it, along with the sharper half: the version-lineage fence is built
// from `rename` rows only, so ANY id reuse after a delete inherits the deleted
// entity's history.
func TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace(t *testing.T) {
	t.Skip("BUG-22XSH3: asserts the headless rule removed by BUG-HC6I2T; " +
		"what should replace it is an open decision")

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
