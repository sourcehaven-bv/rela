package storetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunBulkMigrateTests is the conformance suite for bulk migration rewrites
// (TKT-HH7PKJ).
//
// Every backend must pass these, whether it implements [store.BulkMigrator]
// natively (pgstore and sqlitestore rewrite the table in one statement) or is
// served by the generic fallback. [store.SwapRelationEndpoints] picks the path
// and the OBSERVABLE contract is identical either way — that equivalence is the
// whole point of the seam, and the reason this suite is not gated on a
// capability flag.
//
// Keeping both paths under ONE contract is deliberate. A native fast path and a
// hand-written loop are exactly the pair that drifts: BUG-TMGWIN happened
// because detection and remediation were maintained as two lists, each looking
// complete alone.
func RunBulkMigrateTests(t *testing.T, f Factory) {
	t.Run("SwapRewritesEveryEdgeOfTheType", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A", "B", "C")
		mustRelation(t, s, "A", "blocks", "B", &store.RelationData{
			Properties: map[string]any{"weight": "high"}, Content: "why A blocks B",
		})
		mustRelation(t, s, "A", "blocks", "C", nil)
		// A different type must be untouched: the rewrite is scoped by type.
		mustRelation(t, s, "B", "requires", "C", nil)

		n, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		// Properties and body travel with the edge, not just the endpoints.
		got, err := s.GetRelation(ctx(), "B", "blocks", "A")
		require.NoError(t, err)
		assert.Equal(t, "high", got.Properties["weight"])
		assert.Equal(t, "why A blocks B", got.Content)

		_, err = s.GetRelation(ctx(), "C", "blocks", "A")
		assert.NoError(t, err)
		_, err = s.GetRelation(ctx(), "A", "blocks", "B")
		assert.ErrorIs(t, err, store.ErrNotFound, "the original direction must be gone")
		_, err = s.GetRelation(ctx(), "B", "requires", "C")
		assert.NoError(t, err, "a relation of another type must be untouched")
	})

	t.Run("SelfEdgeSurvivesAndIsNotCounted", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A")
		mustRelation(t, s, "A", "blocks", "A", &store.RelationData{Content: "keep me"})

		n, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)
		assert.Equal(t, 0, n, "reversal is identity for a self-edge, so it rewrites nothing")

		// The regression this pins: a create-then-delete loop conflicts on the
		// create and then deletes the only copy, destroying the edge.
		got, err := s.GetRelation(ctx(), "A", "blocks", "A")
		require.NoError(t, err, "the self-edge must survive")
		assert.Equal(t, "keep me", got.Content)
	})

	t.Run("BothDirectionsIsRefusedAndChangesNothing", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A", "B")
		mustRelation(t, s, "A", "blocks", "B", &store.RelationData{Content: "AB"})
		mustRelation(t, s, "B", "blocks", "A", &store.RelationData{Content: "BA"})

		_, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.Error(t, err, "swapping a both-directions pair would merge two distinct edges")
		assert.ErrorIs(t, err, store.ErrConflict)

		// All-or-nothing: a half-applied rewrite is the failure mode that loses
		// an edge and leaves the survivor carrying the other's content.
		ab, err := s.GetRelation(ctx(), "A", "blocks", "B")
		require.NoError(t, err)
		assert.Equal(t, "AB", ab.Content)
		ba, err := s.GetRelation(ctx(), "B", "blocks", "A")
		require.NoError(t, err)
		assert.Equal(t, "BA", ba.Content)
	})

	// A cycle is the case where a naive loop can rewrite into a key it has not
	// read yet. Every edge's mirror is free, so the swap is legal, and each
	// edge must keep its OWN payload rather than inheriting a neighbour's.
	t.Run("CycleReversesWithoutCrossingPayloads", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A", "B", "C")
		for _, e := range [][2]string{{"A", "B"}, {"B", "C"}, {"C", "A"}} {
			mustRelation(t, s, e[0], "blocks", e[1], &store.RelationData{Content: e[0] + e[1]})
		}

		n, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)
		assert.Equal(t, 3, n)

		for _, want := range []struct{ from, to, content string }{
			{"B", "A", "AB"}, {"C", "B", "BC"}, {"A", "C", "CA"},
		} {
			got, gErr := s.GetRelation(ctx(), want.from, "blocks", want.to)
			require.NoError(t, gErr, "%s--blocks--%s missing", want.from, want.to)
			assert.Equal(t, want.content, got.Content,
				"%s--blocks--%s carries the wrong payload", want.from, want.to)
		}
	})

	// The suite exists because a native fast path and a hand-written loop
	// drift, and EVENTS are where they drift invisibly: final row state can
	// agree while the change feed tells three different stories. A reversal
	// removes the old triple and adds its mirror, so that is what every
	// backend must say — an "updated" on the new triple alone would leave an
	// id-keyed consumer holding a ghost edge in the old direction.
	t.Run("SwapEmitsDeleteThenCreatePerEdge", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A", "B")
		mustRelation(t, s, "A", "blocks", "B", nil)

		events, cancel := s.Subscribe(16)
		defer cancel()

		_, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)

		// Asserted as a SET, not a sequence. The fallback creates before it
		// deletes so a crash between the two duplicates rather than destroys;
		// the native path has no such window and reports the removal first.
		// Both orderings are honest, and pinning one would force a backend to
		// take a worse write order to satisfy a test.
		got := drainRelationEvents(t, events, 2)
		require.Len(t, got, 2, "want one delete and one create")
		var deleted, created *store.Event
		for i := range got {
			switch got[i].Op {
			case store.EventRelationDeleted:
				deleted = &got[i]
			case store.EventRelationCreated:
				created = &got[i]
			case store.EventRelationUpdated:
				t.Errorf("a reversal must not report an update: the old triple ceased to "+
					"exist and the new one did not exist before (%v)", got[i])
			case store.EventEntityCreated, store.EventEntityUpdated, store.EventEntityDeleted:
				// Filtered out by drainRelationEvents; unreachable here.
			}
		}
		require.NotNil(t, deleted, "no delete event for the old direction: %v", got)
		require.NotNil(t, created, "no create event for the new direction: %v", got)
		assert.Equal(t, "A", deleted.From)
		assert.Equal(t, "B", deleted.To)
		assert.Equal(t, "B", created.From)
		assert.Equal(t, "A", created.To)
	})

	// A rewritten row must not still look settled. Staleness heuristics read
	// UpdatedAt — on the database backends the version sweep selects its
	// candidates by it, and a reversal is captured by no synchronous hook, so a
	// row whose timestamp never moved could have its new triple recorded
	// nowhere (the TKT-9TQ6I trap, whose "a miss costs only the marker"
	// reasoning does not apply here).
	t.Run("SwapMovesUpdatedAt", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A", "B")
		mustRelation(t, s, "A", "blocks", "B", nil)
		before, err := s.GetRelation(ctx(), "A", "blocks", "B")
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond) // clocks with coarse resolution
		_, err = store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)

		after, err := s.GetRelation(ctx(), "B", "blocks", "A")
		require.NoError(t, err)
		assert.True(t, after.UpdatedAt.After(before.UpdatedAt),
			"UpdatedAt did not move: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	})

	t.Run("EmptyTypeIsANoOp", func(t *testing.T) {
		s := f(t)
		n, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("ReRunReversesAgainRatherThanConverging", func(t *testing.T) {
		s := f(t)
		seedSwapEntities(t, s, "A", "B")
		mustRelation(t, s, "A", "blocks", "B", nil)

		_, err := store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)
		_, err = store.SwapRelationEndpoints(ctx(), s, "blocks")
		require.NoError(t, err)

		// This documents that the STORE operation is a mechanical swap with no
		// memory, NOT an idempotent migration step. Running it twice returns
		// the data to its starting point, which is why the caller
		// (datamigration) owns the directional test that decides whether an
		// edge still needs reversing. Asserting it here stops a future
		// "optimization" from adding a hidden marker down where the store
		// cannot know what the schema means.
		_, err = s.GetRelation(ctx(), "A", "blocks", "B")
		assert.NoError(t, err, "two swaps return the edge to its original direction")
	})
}

// drainRelationEvents collects up to n relation events, or fails on timeout.
//
// Filters to relation ops: a backend may also emit entity events for the same
// write, and the contract under test is about the relation feed.
func drainRelationEvents(t *testing.T, events <-chan store.Event, n int) []store.Event {
	t.Helper()
	var out []store.Event
	deadline := time.After(5 * time.Second)
	for len(out) < n {
		select {
		case ev := <-events:
			switch ev.Op {
			case store.EventRelationCreated, store.EventRelationUpdated, store.EventRelationDeleted:
				out = append(out, ev)
			default:
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %d relation event(s), got %d: %v", n, len(out), out)
		}
	}
	return out
}

// seedSwapEntities creates the entities the relation tests point at.
func seedSwapEntities(t *testing.T, s store.Store, ids ...string) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "feature")))
	}
}

// mustRelation creates a relation or fails the test.
func mustRelation(t *testing.T, s store.Store, from, relType, to string, data *store.RelationData) {
	t.Helper()
	_, err := s.CreateRelation(ctx(), from, relType, to, data)
	require.NoError(t, err)
}
