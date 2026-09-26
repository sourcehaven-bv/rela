package storetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunSweepAttributionTests pins who a swept create/update version is
// attributed to. [RunVersionTests] drives capture through the synchronous
// writer and so cannot see this; the sweep is the only path that records
// creates and updates, so the attribution it stamps is what history shows for
// almost every edit.
//
// The contract (TKT-ZIRMGM): a write carrying a [store.Attribution] is
// credited to that editor, and a write without one is credited to the
// version-sweep system principal, never to a guessed or stale identity.
//
// sweepNow must run one tick that treats every row as settled.
func RunSweepAttributionTests(t *testing.T, f Factory, sweepNow func(t *testing.T, s store.Store)) {
	attributed := func(user, tool string) context.Context {
		return store.WithAttribution(context.Background(), store.Attribution{User: user, Tool: tool})
	}
	newEntity := func(id, title string) *entity.Entity {
		e := entity.New(id, "feature")
		e.SetString("title", title)
		return e
	}
	requireEntityAuthor := func(t *testing.T, v store.VersionService, id string, ord int, user, tool string) {
		t.Helper()
		metas, err := v.ListVersions(ctx(), id)
		require.NoError(t, err)
		require.Len(t, metas, ord, "versions of %s", id)
		require.Equal(t, user, metas[ord-1].PrincipalUser, "user of %s v%d", id, ord)
		require.Equal(t, tool, metas[ord-1].PrincipalTool, "tool of %s v%d", id, ord)
	}

	t.Run("CreateAndUpdateCreditTheEditor", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		require.NoError(t, s.CreateEntity(attributed("alice", "mcp"), newEntity("FEAT-1", "first")))
		sweepNow(t, s)
		requireEntityAuthor(t, v, "FEAT-1", 1, "alice", "mcp")

		require.NoError(t, s.UpdateEntity(attributed("bob", "data-entry"), newEntity("FEAT-1", "second")))
		sweepNow(t, s)
		requireEntityAuthor(t, v, "FEAT-1", 2, "bob", "data-entry")
	})

	t.Run("UnattributedWriteFallsBackToTheSweep", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		require.NoError(t, s.CreateEntity(attributed("alice", "mcp"), newEntity("FEAT-1", "first")))
		sweepNow(t, s)

		// The unattributed update must not inherit alice: she did not write
		// these bytes.
		require.NoError(t, s.UpdateEntity(ctx(), newEntity("FEAT-1", "second")))
		sweepNow(t, s)
		requireEntityAuthor(t, v, "FEAT-1", 2, "", store.SweepPrincipalTool)
	})

	t.Run("RelationCreateAndUpdateCreditTheEditor", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-1", "from")))
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-2", "to")))
		_, err := s.CreateRelation(attributed("carol", "mcp"), "FEAT-1", "depends-on", "FEAT-2",
			&store.RelationData{Content: "why"})
		require.NoError(t, err)
		sweepNow(t, s)

		q := store.RelationHistoryQuery{From: "FEAT-1", Type: "depends-on", To: "FEAT-2"}
		metas, err := v.ListRelationVersions(ctx(), q)
		require.NoError(t, err)
		require.Len(t, metas, 1)
		require.Equal(t, "carol", metas[0].PrincipalUser)
		require.Equal(t, "mcp", metas[0].PrincipalTool)

		_, err = s.UpdateRelation(ctx(), "FEAT-1", "depends-on", "FEAT-2",
			store.RelationData{Content: "changed"})
		require.NoError(t, err)
		sweepNow(t, s)
		metas, err = v.ListRelationVersions(ctx(), q)
		require.NoError(t, err)
		require.Len(t, metas, 2)
		require.Empty(t, metas[1].PrincipalUser)
		require.Equal(t, store.SweepPrincipalTool, metas[1].PrincipalTool)
	})

	// A reversal rewrites the triple in bulk, outside CreateRelation and
	// UpdateRelation, so it must stamp the editor itself. Otherwise the swept
	// version of the reversed edge names whoever last edited it before.
	t.Run("EndpointSwapCreditsTheSwapper", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			swapper    store.Attribution
			user, tool string
		}{
			{"Attributed", store.Attribution{User: "dave", Tool: "data-migration"}, "dave", "data-migration"},
			{"Unattributed", store.Attribution{}, "", store.SweepPrincipalTool},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := f(t)
				v := versionsOf(t, s)
				require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-1", "from")))
				require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-2", "to")))
				_, err := s.CreateRelation(attributed("carol", "mcp"), "FEAT-1", "depends-on", "FEAT-2", nil)
				require.NoError(t, err)
				sweepNow(t, s)

				swapCtx := ctx()
				if !tc.swapper.IsZero() {
					swapCtx = attributed(tc.swapper.User, tc.swapper.Tool)
				}
				n, err := store.SwapRelationEndpoints(swapCtx, s, "depends-on")
				require.NoError(t, err)
				require.Equal(t, 1, n)
				sweepNow(t, s)

				metas, err := v.ListRelationVersions(ctx(),
					store.RelationHistoryQuery{From: "FEAT-2", Type: "depends-on", To: "FEAT-1"})
				require.NoError(t, err)
				require.NotEmpty(t, metas)
				last := metas[len(metas)-1]
				require.Equal(t, tc.user, last.PrincipalUser)
				require.Equal(t, tc.tool, last.PrincipalTool)
			})
		}
	})
}
