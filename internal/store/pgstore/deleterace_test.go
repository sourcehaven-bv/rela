package pgstore_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestDeleteEntity_RacingStateCreateIsAtomicPerFamily exercises a family
// delete against a concurrent state create and asserts the graph is left
// consistent: nothing may survive holding edges the delete already swept.
//
// It used to assert that NO row of the id survives, which held because of the
// HEADLESS-STATE rule — a create landing after the delete found no
// zero-coordinate row and was refused. BUG-HC6I2T removed that rule, so the
// refusal is gone and that assertion no longer describes the code: a create
// taking the lock after the delete commits legitimately mints a NEW family
// under the freed id. Verified directly — sequential create/delete/create
// leaves exactly `[PAGE-XX@draft]`.
//
// # What this test does and does not prove
//
// It pins graph consistency, and it runs unskipped so the Postgres Backend job
// (which fails on any `--- SKIP`) gets a real result.
//
// It is NOT a proof that lockFamily works. Removing the delete-side
// lockFamily call leaves this test GREEN, because 25 rounds do not reliably
// hit the interleaving window. Do not read a pass here as the lock being
// exercised; a deterministic version needs an injected barrier between the
// delete's family scan and its sweep, which the store offers no seam for.
// BUG-22XSH3 carries that, along with the semantic half — whether id reuse
// after a delete should be refused at all, and the version-lineage fence
// (built from `rename` rows only) that lets ANY reused id inherit the deleted
// entity's history.
func TestDeleteEntity_RacingStateCreateIsAtomicPerFamily(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	const rounds = 25
	for i := range rounds {
		id := "PAGE-" + string(rune('A'+i%26)) + string(rune('A'+i/26))
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "page")))

		// An inbound edge, so the amputation this lock prevents is
		// OBSERVABLE: the delete sweeps `from_id = id OR to_id = id`, so a
		// face surviving with its edge already gone is the corruption.
		// Without seeding this the graph assertion below could never fail.
		peer := "PEER-" + string(rune('A'+i%26)) + string(rune('A'+i/26))
		require.NoError(t, s.CreateEntity(ctx, entity.New(peer, "page")))
		_, relErr := s.CreateRelation(ctx, peer, "links", id, nil)
		require.NoError(t, relErr)

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
			_, _ = s.DeleteEntity(ctx, id, true)
		}()
		wg.Wait()

		// Whatever the order, the delete must be ATOMIC over the family. A
		// half-swept family — the bare row gone and the draft left behind, or
		// vice versa — is the corruption lockFamily prevents. Either outcome
		// is legitimate: the delete won (nothing left), or the create won the
		// lock and re-established the family (its own rows, intact).
		var left []string
		for e, err := range s.ListEntities(ctx, store.EntityQuery{IDs: []string{id}, AllStates: true}) {
			require.NoError(t, err)
			left = append(left, e.ID+"@"+e.Face.String())
		}
		// The guarantee lockFamily documents is about the GRAPH, not the row
		// count: a surviving face must never be left holding edges the delete
		// already swept ("the survivor looks intact while its graph has been
		// amputated"). Emptiness is NOT the invariant — a create that takes
		// the lock after the delete commits legitimately mints a new family
		// under the freed id, and `[PAGE-XX@draft]` alone is exactly that
		// (verified directly: sequential create-delete-create leaves precisely
		// that set).
		//
		// So: whatever survived, its edges must be consistent with it. The
		// round seeds an inbound edge before racing, so a survivor that kept
		// rows while its edge vanished is the amputation this lock prevents.
		var edges int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM relations WHERE from_id = $1 OR to_id = $1`,
			id).Scan(&edges))
		if len(left) == 0 {
			require.Zerof(t, edges,
				"round %d: family deleted but %d edge(s) survived", i, edges)
		} else if slices.Contains(left, id+"@") {
			// The ORIGINAL family survived (it holds the zero coordinate this
			// round seeded, along with the edge). Its edge must have survived
			// with it; a surviving original whose edge was swept is exactly
			// the amputated graph lockFamily documents.
			require.NotZerof(t, edges,
				"round %d: the seeded family survived (%v) but its edge was "+
					"swept — the delete amputated a family it then left standing",
				i, left)
		}
	}
}
