package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

func newTombstoneStore(t *testing.T) *pgstore.Store {
	t.Helper()
	pool := newScopedPool(t)
	st, err := pgstore.New(pool)
	require.NoError(t, err)
	return st
}

func mustCreateEntity(t *testing.T, st *pgstore.Store, id, typ string) {
	t.Helper()
	e := entity.New(id, typ)
	e.SetString("title", "t")
	require.NoError(t, st.CreateEntity(context.Background(), e))
}

// TestDeleteWritesEntityTombstone: deleting an entity records a tombstone with a
// fresh seq, and that tombstone surfaces in the manifest after the live row is
// gone.
func TestDeleteWritesEntityTombstone(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "REQ-1", "requirement")
	before, err := st.ManifestSince(ctx, 0)
	require.NoError(t, err)
	// cursor just below the create so the delete is clearly past it
	cursor := before[len(before)-1].Seq

	_, err = st.DeleteEntity(ctx, "REQ-1", false)
	require.NoError(t, err)

	entries, err := st.ManifestSince(ctx, cursor)
	require.NoError(t, err)
	require.Len(t, entries, 1, "expected exactly the delete tombstone since the create")
	tomb := entries[0]
	require.True(t, tomb.Deleted, "entry must be a tombstone")
	require.Equal(t, "e", tomb.Kind)
	require.Equal(t, "REQ-1", tomb.IDA)
	require.Equal(t, "requirement", tomb.Typ)
	require.Greater(t, tomb.Seq, cursor, "tombstone seq must advance past the cursor")
}

// TestDeleteWritesRelationTombstone: deleting a relation records a tombstone
// carrying the full from/type/to triple.
func TestDeleteWritesRelationTombstone(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "DEC-1", "decision")
	mustCreateEntity(t, st, "REQ-1", "requirement")
	_, err := st.CreateRelation(ctx, "DEC-1", "addresses", "REQ-1", &store.RelationData{})
	require.NoError(t, err)

	pre, err := st.ManifestSince(ctx, 0)
	require.NoError(t, err)
	cursor := pre[len(pre)-1].Seq

	require.NoError(t, st.DeleteRelation(ctx, "DEC-1", "addresses", "REQ-1"))

	entries, err := st.ManifestSince(ctx, cursor)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	tomb := entries[0]
	require.True(t, tomb.Deleted)
	require.Equal(t, "r", tomb.Kind)
	require.Equal(t, "DEC-1", tomb.IDA)
	require.Equal(t, "addresses", tomb.IDB)
	require.Equal(t, "REQ-1", tomb.IDC)
}

// TestCascadeDeleteTombstonesRelations: deleting an entity with cascade writes a
// tombstone for the entity AND each cascaded relation.
func TestCascadeDeleteTombstonesRelations(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "DEC-1", "decision")
	mustCreateEntity(t, st, "REQ-1", "requirement")
	_, err := st.CreateRelation(ctx, "DEC-1", "addresses", "REQ-1", &store.RelationData{})
	require.NoError(t, err)

	pre, err := st.ManifestSince(ctx, 0)
	require.NoError(t, err)
	cursor := pre[len(pre)-1].Seq

	_, err = st.DeleteEntity(ctx, "DEC-1", true) // cascade
	require.NoError(t, err)

	entries, err := st.ManifestSince(ctx, cursor)
	require.NoError(t, err)

	var entTomb, relTomb int
	for _, e := range entries {
		require.True(t, e.Deleted)
		switch e.Kind {
		case "e":
			entTomb++
		case "r":
			relTomb++
		}
	}
	require.Equal(t, 1, entTomb, "one entity tombstone")
	require.Equal(t, 1, relTomb, "one relation tombstone for the cascaded relation")
}

// TestManifestDeleteThenRecreate: deleting then recreating the same id yields a
// tombstone AND the new live row (the recreate), in seq order.
//
// The original create's live row is GONE (the recreate is a new row at the same
// primary key, with a higher seq), so the manifest from seq 0 shows the
// tombstone for the delete plus the current live row — the original create
// leaves no trace at its old seq. That is correct: a manifest reflects current
// live rows + all tombstones, not historical row versions.
func TestManifestDeleteThenRecreate(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "REQ-1", "requirement")
	_, err := st.DeleteEntity(ctx, "REQ-1", false)
	require.NoError(t, err)
	mustCreateEntity(t, st, "REQ-1", "requirement") // recreate same id

	entries, err := st.ManifestSince(ctx, 0)
	require.NoError(t, err)

	// In seq order: tombstone(delete) then live(recreate). The original create's
	// row was overwritten by the recreate (same PK), so it is not present.
	require.Len(t, entries, 2)
	require.True(t, entries[0].Deleted, "first: delete tombstone")
	require.False(t, entries[1].Deleted, "second: recreate (current live row)")
	require.Equal(t, "REQ-1", entries[0].IDA)
	require.Equal(t, "REQ-1", entries[1].IDA)
	require.Less(t, entries[0].Seq, entries[1].Seq, "tombstone precedes the recreate in seq order")
}

// TestRenameTombstonesOldIdentities is the regression for the code-review
// finding: a rename re-keys the entity and its relations in place, so to an
// id-keyed sync client the OLD id and OLD relation triples are removed. They
// must be tombstoned, or the client keeps ghost entities/edges forever.
func TestRenameTombstonesOldIdentities(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "DEC-1", "decision")
	mustCreateEntity(t, st, "REQ-1", "requirement")
	_, err := st.CreateRelation(ctx, "DEC-1", "addresses", "REQ-1", &store.RelationData{})
	require.NoError(t, err)

	pre, err := st.ManifestSince(ctx, 0)
	require.NoError(t, err)
	cursor := pre[len(pre)-1].Seq

	_, err = st.RenameEntity(ctx, "DEC-1", "DEC-2")
	require.NoError(t, err)

	entries, err := st.ManifestSince(ctx, cursor)
	require.NoError(t, err)

	// Expect, among the entries since the cursor:
	//   - a tombstone for entity DEC-1 (the old id)
	//   - a tombstone for the old relation triple (DEC-1, addresses, REQ-1)
	//   - a live row for DEC-2 (the re-keyed entity)
	//   - a live row for the re-keyed relation (DEC-2, addresses, REQ-1)
	var entTomb, relTomb, liveNewEnt, liveNewRel bool
	for _, e := range entries {
		switch {
		case e.Deleted && e.Kind == "e" && e.IDA == "DEC-1":
			entTomb = true
		case e.Deleted && e.Kind == "r" && e.IDA == "DEC-1" && e.IDC == "REQ-1":
			relTomb = true
		case !e.Deleted && e.Kind == "e" && e.IDA == "DEC-2":
			liveNewEnt = true
		case !e.Deleted && e.Kind == "r" && e.IDA == "DEC-2" && e.IDC == "REQ-1":
			liveNewRel = true
		}
	}
	require.True(t, entTomb, "rename must tombstone the old entity id DEC-1")
	require.True(t, relTomb, "rename must tombstone the old relation triple (DEC-1,addresses,REQ-1)")
	require.True(t, liveNewEnt, "the re-keyed entity DEC-2 must appear live")
	require.True(t, liveNewRel, "the re-keyed relation must appear live")
}

// TestSeqIndexesExist verifies migration 0003 created the seq B-tree indexes
// the manifest/catch-up "WHERE seq > X ORDER BY seq" scans depend on. (Whether
// the planner *uses* them is data-size dependent and not asserted here — on a
// tiny test table the planner correctly prefers a seqscan; the durable
// guarantee is that the indexes exist for production-scale tables.)
func TestSeqIndexesExist(t *testing.T) {
	pool := newScopedPool(t)
	// Scope to this test's own (search_path-resolved) schema — the same index
	// names may also exist in `public` from other migrations, so an unscoped
	// count would be misleading.
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_indexes
		 WHERE schemaname = current_schema()
		   AND indexname IN ('entities_seq_idx','relations_seq_idx','deletions_seq_idx')`).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 3, n, "all three seq indexes must exist in this schema after migration")
}

// TestCatchUpRecoversMissedDelete is the headline regression: with the live
// NOTIFY disabled, a delete on writer A reaches writer B ONLY via the safety-net
// catch-up. Before tombstones the catch-up was delete-blind (it scanned live
// rows, which no longer include the deleted record), so the deletion was lost.
// Now the catch-up scans the deletions table and emits the Delete event.
func TestCatchUpRecoversMissedDelete(t *testing.T) {
	schema := freshFeedSchema(t)
	// Fast safety-net catch-up so the test doesn't wait the default interval.
	pgstore.SetCatchUpIntervalForTest(t, 100*time.Millisecond)

	a := openWriter(t, schema)
	b := openWriter(t, schema)

	// Seed an entity (live NOTIFY still on) and let b observe it so its
	// watermark is primed past the create.
	ch, cancel := b.Subscribe(64)
	defer cancel()
	ctx := context.Background()
	seed := entity.New("REQ-1", "requirement")
	seed.SetString("title", "t")
	require.NoError(t, a.CreateEntity(ctx, seed))
	waitForEntityEvent(t, ch, "REQ-1", 5*time.Second)

	// Now disable the live NOTIFY so the delete can ONLY arrive via catch-up.
	pgstore.SetNotifyDisabledForTest(t, true)

	_, err := a.DeleteEntity(ctx, "REQ-1", false)
	require.NoError(t, err)

	// The catch-up must deliver a Delete event for REQ-1.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.EntityID == "REQ-1" && ev.Op == store.EventEntityDeleted {
				return // recovered the missed delete
			}
		case <-deadline:
			t.Fatal("catch-up did not recover the missed delete (delete-blind regression)")
		}
	}
}
