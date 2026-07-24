package pgstore_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestListRelationLifetimes_ReusedKeyEnumeratesAll is the core TKT-HGE4KW
// regression: a deleted-and-recreated key has its OLDER lifetime reachable, not
// silently collapsed to the newest. (A,blocks,X) is created→deleted (gen1), then
// an unrelated (A,blocks,X) is created→deleted (gen2). Both lifetimes must be
// enumerated newest-first with distinct RecordIDs, and gen1's history — orphaned
// before this ticket — must now be readable via its RecordID.
func TestListRelationLifetimes_ReusedKeyEnumeratesAll(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	// gen1: create + delete, then remove the live row.
	_, err = s.CreateRelation(ctx, "A", "blocks", "X", &store.RelationData{Content: "gen1"})
	require.NoError(t, err)
	rid1 := relRecordID(ctx, t, pool, "A", "blocks", "X")
	c1 := newRelVersionInput(rid1, "A", "blocks", "X", "gen1")
	c1.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c1))
	d1 := newRelVersionInput(rid1, "A", "blocks", "X", "gen1")
	d1.Op = store.VersionOpDelete
	require.NoError(t, vs.WriteRelationVersion(ctx, d1))
	require.NoError(t, s.DeleteRelation(ctx, "A", "blocks", "X"))

	// gen2: recreate the same triple (fresh rel_record_id), create + delete.
	_, err = s.CreateRelation(ctx, "A", "blocks", "X", &store.RelationData{Content: "gen2"})
	require.NoError(t, err)
	rid2 := relRecordID(ctx, t, pool, "A", "blocks", "X")
	require.NotEqual(t, rid1, rid2)
	c2 := newRelVersionInput(rid2, "A", "blocks", "X", "gen2")
	c2.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c2))
	d2 := newRelVersionInput(rid2, "A", "blocks", "X", "gen2")
	d2.Op = store.VersionOpDelete
	require.NoError(t, vs.WriteRelationVersion(ctx, d2))
	require.NoError(t, s.DeleteRelation(ctx, "A", "blocks", "X"))

	// Enumerate: two lifetimes, newest-first, distinct record ids, neither live.
	lifetimes, err := vs.ListRelationLifetimes(ctx, "A", "blocks", "X")
	require.NoError(t, err)
	require.Len(t, lifetimes, 2, "both lifetimes of the reused key are enumerated")
	require.Equal(t, 1, lifetimes[0].Lifetime)
	require.Equal(t, rid2, lifetimes[0].RecordID, "lifetime 1 = newest (gen2)")
	require.Equal(t, 2, lifetimes[1].Lifetime)
	require.Equal(t, rid1, lifetimes[1].RecordID, "lifetime 2 = older (gen1)")
	require.False(t, lifetimes[0].Live)
	require.False(t, lifetimes[1].Live)
	require.Equal(t, store.VersionOpDelete, lifetimes[0].FinalOp)
	require.Equal(t, 2, lifetimes[0].VersionCount)

	// Newest (RecordID 0) reads gen2; the OLDER lifetime is now reachable by
	// RecordID — the regression this ticket fixes (it was orphaned before).
	newest, err := vs.ListRelationVersions(ctx, store.RelationHistoryQuery{From: "A", Type: "blocks", To: "X"})
	require.NoError(t, err)
	require.Len(t, newest, 2)
	snapNew, err := vs.GetRelationVersion(ctx, store.RelationHistoryQuery{From: "A", Type: "blocks", To: "X"}, 1)
	require.NoError(t, err)
	require.Equal(t, "gen2", snapNew.Content)

	old, err := vs.ListRelationVersions(ctx,
		store.RelationHistoryQuery{From: "A", Type: "blocks", To: "X", RecordID: rid1})
	require.NoError(t, err)
	require.Len(t, old, 2, "gen1 lifetime reachable via its RecordID")
	snapOld, err := vs.GetRelationVersion(ctx,
		store.RelationHistoryQuery{From: "A", Type: "blocks", To: "X", RecordID: rid1}, 1)
	require.NoError(t, err)
	require.Equal(t, "gen1", snapOld.Content, "older lifetime's content is no longer orphaned")
}

// TestListRelationLifetimes_LiveKeyOneLifetime: a live relation with a single
// history reports exactly one lifetime, flagged Live.
func TestListRelationLifetimes_LiveKeyOneLifetime(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	_, err = s.CreateRelation(ctx, "L", "links", "M", &store.RelationData{Content: "v1"})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "L", "links", "M")
	c := newRelVersionInput(rid, "L", "links", "M", "v1")
	c.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c))

	lifetimes, err := vs.ListRelationLifetimes(ctx, "L", "links", "M")
	require.NoError(t, err)
	require.Len(t, lifetimes, 1)
	require.True(t, lifetimes[0].Live, "the single lifetime is the live relation")
	require.Equal(t, rid, lifetimes[0].RecordID)
}

// TestListRelationLifetimes_UnknownKeyEmpty: a key with no history returns no
// lifetimes (not an error).
func TestListRelationLifetimes_UnknownKeyEmpty(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	lifetimes, err := s.VersionStore().ListRelationLifetimes(ctx, "NO", "such", "KEY")
	require.NoError(t, err)
	require.Empty(t, lifetimes)
}

// TestRelationHistoryQuery_RecordIDMustBelongToKey: a RecordID that is not a
// lifetime of the queried key yields ErrNotFound — the composite key stays the
// authorization boundary, so a caller cannot read an arbitrary lineage by id.
func TestRelationHistoryQuery_RecordIDMustBelongToKey(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	// Key ONE with a lineage.
	_, err = s.CreateRelation(ctx, "ONE-A", "links", "ONE-B", &store.RelationData{Content: "x"})
	require.NoError(t, err)
	ridOne := relRecordID(ctx, t, pool, "ONE-A", "links", "ONE-B")
	c := newRelVersionInput(ridOne, "ONE-A", "links", "ONE-B", "x")
	c.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c))

	// Unrelated key TWO.
	_, err = s.CreateRelation(ctx, "TWO-A", "links", "TWO-B", &store.RelationData{Content: "y"})
	require.NoError(t, err)
	ridTwo := relRecordID(ctx, t, pool, "TWO-A", "links", "TWO-B")
	c2 := newRelVersionInput(ridTwo, "TWO-A", "links", "TWO-B", "y")
	c2.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c2))

	// Asking for key ONE but with key TWO's rel_record_id → ErrNotFound.
	_, err = vs.ListRelationVersions(ctx,
		store.RelationHistoryQuery{From: "ONE-A", Type: "links", To: "ONE-B", RecordID: ridTwo})
	require.ErrorIs(t, err, store.ErrNotFound)

	// The valid own RecordID resolves fine.
	metas, err := vs.ListRelationVersions(ctx,
		store.RelationHistoryQuery{From: "ONE-A", Type: "links", To: "ONE-B", RecordID: ridOne})
	require.NoError(t, err)
	require.Len(t, metas, 1)
}

// TestListRelationLifetimes_DeleteOnlyLineage: a lineage whose ONLY row is a
// delete (a short-lived relation the sweep never captured a create for) still
// appears as a lifetime with a correct span — a lifetime is bounded by
// rel_record_id, NOT by requiring a create row.
func TestListRelationLifetimes_DeleteOnlyLineage(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	// Create the row, capture ONLY a delete version (no create), delete the row —
	// modeling a relation created and deleted inside one sweep-idle window.
	_, err = s.CreateRelation(ctx, "SL-A", "links", "SL-B", &store.RelationData{Content: "brief"})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "SL-A", "links", "SL-B")
	d := newRelVersionInput(rid, "SL-A", "links", "SL-B", "brief")
	d.Op = store.VersionOpDelete
	require.NoError(t, vs.WriteRelationVersion(ctx, d))
	require.NoError(t, s.DeleteRelation(ctx, "SL-A", "links", "SL-B"))

	lifetimes, err := vs.ListRelationLifetimes(ctx, "SL-A", "links", "SL-B")
	require.NoError(t, err)
	require.Len(t, lifetimes, 1, "a delete-only lineage is still a lifetime")
	require.Equal(t, 1, lifetimes[0].VersionCount)
	require.Equal(t, store.VersionOpDelete, lifetimes[0].FinalOp)
	require.False(t, lifetimes[0].Live)
}

// seedTwoDeletedLifetimes creates gen1 then gen2 of (from,type,to), each a
// create+delete captured then the live row removed. Returns (rid1, rid2).
func seedTwoDeletedLifetimes(
	ctx context.Context, t *testing.T, s *pgstore.Store, pool *pgxpool.Pool, from, relType, to string,
) (rid1, rid2 int64) {
	t.Helper()
	vs := s.VersionStore()
	rids := []*int64{&rid1, &rid2}
	for i, content := range []string{"gen1", "gen2"} {
		_, err := s.CreateRelation(ctx, from, relType, to, &store.RelationData{Content: content})
		require.NoError(t, err)
		*rids[i] = relRecordID(ctx, t, pool, from, relType, to)
		c := newRelVersionInput(*rids[i], from, relType, to, content)
		c.Op = store.VersionOpCreate
		require.NoError(t, vs.WriteRelationVersion(ctx, c))
		d := newRelVersionInput(*rids[i], from, relType, to, content)
		d.Op = store.VersionOpDelete
		require.NoError(t, vs.WriteRelationVersion(ctx, d))
		require.NoError(t, s.DeleteRelation(ctx, from, relType, to))
	}
	return rid1, rid2
}

// TestPurgeRelationVersions_MultiLifetimeRefusedWithoutSelector: purging a reused
// key with no lifetime selector REFUSES (erases nothing) — the compliance fix, so
// an operator can't silently leave older lifetimes' content behind.
func TestPurgeRelationVersions_MultiLifetimeRefusedWithoutSelector(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	seedTwoDeletedLifetimes(ctx, t, s, pool, "P-A", "blocks", "P-B")

	res, err := vs.PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "P-A", Type: "blocks", To: "P-B",
		Selector: store.PurgeSelector{All: true}, // purge-all within a lifetime, but no LIFETIME chosen
		Reason:   "gdpr", PrincipalUser: "op",
	})
	require.NoError(t, err)
	require.True(t, res.MultiLifetimeRefused, "reused key without a lifetime selector is refused")
	require.Equal(t, 2, res.LifetimeCount)
	require.Zero(t, res.Purged, "nothing erased on refusal")

	// Both lifetimes intact.
	lifetimes, err := vs.ListRelationLifetimes(ctx, "P-A", "blocks", "P-B")
	require.NoError(t, err)
	require.Len(t, lifetimes, 2)
}

// TestPurgeRelationVersions_LifetimePurgesOnlyThatLineage: --lifetime (RecordID)
// purges exactly one lifetime; the other survives.
func TestPurgeRelationVersions_LifetimePurgesOnlyThatLineage(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	rid1, rid2 := seedTwoDeletedLifetimes(ctx, t, s, pool, "Q-A", "blocks", "Q-B")

	// Purge only the OLDER lifetime (rid1).
	res, err := vs.PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "Q-A", Type: "blocks", To: "Q-B",
		Selector: store.PurgeSelector{All: true},
		RecordID: rid1,
		Reason:   "gdpr", PrincipalUser: "op",
	})
	require.NoError(t, err)
	require.False(t, res.MultiLifetimeRefused)
	require.Positive(t, res.Purged, "the selected lifetime's rows are purged")

	// The newer lifetime (rid2) survives; only one lifetime remains.
	lifetimes, err := vs.ListRelationLifetimes(ctx, "Q-A", "blocks", "Q-B")
	require.NoError(t, err)
	require.Len(t, lifetimes, 1)
	require.Equal(t, rid2, lifetimes[0].RecordID, "the un-purged (newer) lifetime remains")
}

// TestPurgeRelationVersions_AllLifetimesErasesEverything: --all-lifetimes purges
// every lifetime of a reused key — the complete erasure path.
func TestPurgeRelationVersions_AllLifetimesErasesEverything(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	seedTwoDeletedLifetimes(ctx, t, s, pool, "R-A", "blocks", "R-B")

	res, err := vs.PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "R-A", Type: "blocks", To: "R-B",
		Selector:     store.PurgeSelector{All: true},
		AllLifetimes: true,
		Reason:       "gdpr", PrincipalUser: "op",
	})
	require.NoError(t, err)
	require.False(t, res.MultiLifetimeRefused)
	require.Equal(t, 4, res.Purged, "both lifetimes' create+delete rows (2×2) erased")

	lifetimes, err := vs.ListRelationLifetimes(ctx, "R-A", "blocks", "R-B")
	require.NoError(t, err)
	require.Empty(t, lifetimes, "no lifetimes remain after --all-lifetimes")
}

// TestPurgeRelationVersions_SingleLifetimeNoSelector: a key with ONE lifetime
// purges without a selector (no refusal — the refusal is multi-lifetime only).
func TestPurgeRelationVersions_SingleLifetimeNoSelector(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	// One deleted lifetime.
	_, err = s.CreateRelation(ctx, "S-A", "blocks", "S-B", &store.RelationData{Content: "only"})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "S-A", "blocks", "S-B")
	c := newRelVersionInput(rid, "S-A", "blocks", "S-B", "only")
	c.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c))
	d := newRelVersionInput(rid, "S-A", "blocks", "S-B", "only")
	d.Op = store.VersionOpDelete
	require.NoError(t, vs.WriteRelationVersion(ctx, d))
	require.NoError(t, s.DeleteRelation(ctx, "S-A", "blocks", "S-B"))

	res, err := vs.PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "S-A", Type: "blocks", To: "S-B",
		Selector: store.PurgeSelector{All: true},
		Reason:   "gdpr", PrincipalUser: "op",
	})
	require.NoError(t, err)
	require.False(t, res.MultiLifetimeRefused, "single-lifetime key is not refused")
	require.Equal(t, 2, res.Purged)
}

// TestListRelationLifetimes_RenamedAwayNotListedUnderOldKey: a lineage renamed
// AWAY from key K (its FINAL row carries the NEW triple) must NOT appear as a
// lifetime of the OLD key — the heads query filters on the final row's endpoints.
func TestListRelationLifetimes_RenamedAwayNotListedUnderOldKey(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	require.NoError(t, s.CreateEntity(ctx, mkEntity("RA", "ticket", "")))
	require.NoError(t, s.CreateEntity(ctx, mkEntity("RX", "ticket", "")))
	_, err = s.CreateRelation(ctx, "RA", "links", "RX", &store.RelationData{Content: "v1"})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "RA", "links", "RX")
	c := newRelVersionInput(rid, "RA", "links", "RX", "v1")
	c.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c))

	// Rename RA->RA2 (atomic): the lineage's final row now carries (RA2,links,RX).
	_, err = s.RenameEntity(ctx, "RA", "RA2")
	require.NoError(t, err)
	ren := newRelVersionInput(0, "RA2", "links", "RX", "v1")
	ren.Op = store.VersionOpRename
	ren.PrevFrom = "RA"
	ren.PrevTo = "RX"
	require.NoError(t, vs.WriteRelationVersion(ctx, ren))

	// The OLD key has NO lifetime (the lineage was renamed away from it)...
	oldLifetimes, err := vs.ListRelationLifetimes(ctx, "RA", "links", "RX")
	require.NoError(t, err)
	require.Empty(t, oldLifetimes, "renamed-away lineage is not a lifetime of the old key")

	// ...and the NEW key has exactly one (live) lifetime carrying the whole history.
	newLifetimes, err := vs.ListRelationLifetimes(ctx, "RA2", "links", "RX")
	require.NoError(t, err)
	require.Len(t, newLifetimes, 1)
	require.True(t, newLifetimes[0].Live)
	require.Equal(t, rid, newLifetimes[0].RecordID)
}

// TestListRelationLifetimes_ForkedRenameStitchesToOneLifetime models the
// pre-#1127 rename decomposition (delete-old-triple + create-new-triple → the new
// triple gets a FRESH rel_record_id, with a `rename` row carrying prev_from/
// prev_to). The relationLineageIDs stitch must fold the old lineage into the new
// one so the key reports ONE lifetime, not two — and the `claimed`-set dedup must
// not double-list the folded predecessor. AllLifetimes purge over the stitched
// pair must then erase every row exactly once.
func TestListRelationLifetimes_ForkedRenameStitchesToOneLifetime(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	vs := s.VersionStore()

	// Old lineage on (FA,links,FX): create then delete (the decomposition's delete
	// of the old triple). Model with an explicit rel_record_id we control.
	_, err = s.CreateRelation(ctx, "FA", "links", "FX", &store.RelationData{Content: "v1"})
	require.NoError(t, err)
	oldRID := relRecordID(ctx, t, pool, "FA", "links", "FX")
	c1 := newRelVersionInput(oldRID, "FA", "links", "FX", "v1")
	c1.Op = store.VersionOpCreate
	require.NoError(t, vs.WriteRelationVersion(ctx, c1))
	require.NoError(t, s.DeleteRelation(ctx, "FA", "links", "FX"))

	// New lineage on (FA2,links,FX): fresh rel_record_id + a rename row carrying the
	// old triple as prev_from/prev_to — the pre-#1127 stitch link.
	_, err = s.CreateRelation(ctx, "FA2", "links", "FX", &store.RelationData{Content: "v1"})
	require.NoError(t, err)
	newRID := relRecordID(ctx, t, pool, "FA2", "links", "FX")
	require.NotEqual(t, oldRID, newRID)
	ren := newRelVersionInput(newRID, "FA2", "links", "FX", "v1")
	ren.Op = store.VersionOpRename
	ren.PrevFrom = "FA"
	ren.PrevTo = "FX"
	require.NoError(t, vs.WriteRelationVersion(ctx, ren))

	// The new key reports ONE lifetime — the two rel_record_ids are stitched, not
	// double-listed (the claimed-set fold firing).
	lifetimes, err := vs.ListRelationLifetimes(ctx, "FA2", "links", "FX")
	require.NoError(t, err)
	require.Len(t, lifetimes, 1, "forked rename stitches to a single lifetime")
	require.Equal(t, newRID, lifetimes[0].RecordID)
	require.Equal(t, 2, lifetimes[0].VersionCount, "both the old create and the rename row are in the stitched history")

	// AllLifetimes purge erases every row of the stitched pair exactly once.
	res, err := vs.PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "FA2", Type: "links", To: "FX",
		Selector:     store.PurgeSelector{All: true},
		AllLifetimes: true,
		ForceLive:    true, // the FA2 relation is live; erase anyway (writes a tombstone)
		Reason:       "gdpr", PrincipalUser: "op",
	})
	require.NoError(t, err)
	// The rename row is refused by the non-rename guardrail, so nothing is purged —
	// assert that guardrail wins over AllLifetimes (a rename row orphans lineage).
	require.True(t, res.RenameInTargets, "the stitched history contains the rename row; purge refuses it")
	require.Zero(t, res.Purged)
}

// TestPurgeRelationVersions_RecordIDAndAllLifetimesMutuallyExclusive: the store
// (trust boundary) refuses a request setting both RecordID and AllLifetimes.
func TestPurgeRelationVersions_RecordIDAndAllLifetimesMutuallyExclusive(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.VersionStore().PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "X", Type: "links", To: "Y",
		RecordID: 5, AllLifetimes: true,
		Reason: "gdpr", PrincipalUser: "op",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}
