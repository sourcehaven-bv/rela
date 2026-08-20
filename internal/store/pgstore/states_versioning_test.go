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

// These tests pin the Step-1 default-world versioning scope (TKT-DOFYR1,
// architect-signed): the sweep's `pointer = ''` / `from_pointer = ''`
// skips are DELIBERATE — a state capture under the bare-id lineage would
// interleave a family's faces (corrupt history purge would have to
// fence), and per-state history is designed with its consumer, the
// Step-4 copy kernel (TKT-C1XUA8).

// TestSweep_SkipsStateRows: a settled STATE row mints no version; its
// settled default face still does — the bare-id lineage stays pure.
func TestSweep_SkipsStateRows(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(ctx, mkEntity("PAGE-1", "ticket", "default face")))
	draft := mkEntity("PAGE-1", "ticket", "draft face")
	p, err := entity.ParsePointer("draft")
	require.NoError(t, err)
	draft.Pointer = p
	require.NoError(t, s.CreateEntity(ctx, draft))

	// Backdate BOTH rows so both would qualify as settled.
	_, err = pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = 'PAGE-1'`)
	require.NoError(t, err)

	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	// The default face is captured exactly once…
	require.Eventually(t, func() bool {
		metas, e := s.VersionStore().ListVersions(ctx, "PAGE-1")
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond, "default face should be captured once")

	// …and STAYS at one: the draft row must never join the lineage. A
	// second interval's worth of ticks would have captured it if the
	// skip were missing (its content differs from the captured hash).
	time.Sleep(300 * time.Millisecond)
	metas, err := s.VersionStore().ListVersions(ctx, "PAGE-1")
	require.NoError(t, err)
	require.Len(t, metas, 1, "a state row must not mint a version under the bare-id lineage")
}

// TestSweep_SkipsStateTailedRelations: a settled state-tailed edge mints
// no relation version, while the default-tail edge of the SAME triple
// does — and the two edges hold distinct rel_record_ids, so lineages
// could never merge even without the skip.
func TestSweep_SkipsStateTailedRelations(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(ctx, mkEntity("PAGE-2", "ticket", "default")))
	p, err := entity.ParsePointer("draft")
	require.NoError(t, err)
	draft := mkEntity("PAGE-2", "ticket", "draft")
	draft.Pointer = p
	require.NoError(t, s.CreateEntity(ctx, draft))
	require.NoError(t, s.CreateEntity(ctx, mkEntity("SPEC-1", "ticket", "target")))

	_, err = s.CreateRelation(ctx, "PAGE-2", "references", "SPEC-1", nil)
	require.NoError(t, err)
	_, err = s.CreateRelation(ctx, "PAGE-2", "references", "SPEC-1",
		&store.RelationData{FromPointer: p})
	require.NoError(t, err)

	// Distinct rel_record_ids for the two tails of one triple.
	var ids []int64
	rows, err := pool.Query(ctx,
		`SELECT rel_record_id FROM relations WHERE from_id = 'PAGE-2' ORDER BY from_pointer`)
	require.NoError(t, err)
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	rows.Close()
	require.Len(t, ids, 2)
	require.NotEqual(t, ids[0], ids[1], "each tail must hold its own lineage id")

	// Backdate everything so both edges would qualify as settled.
	_, err = pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE relations SET updated_at = now() - interval '1 hour'`)
	require.NoError(t, err)

	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	// The default-tail edge is captured…
	require.Eventually(t, func() bool {
		metas, e := s.VersionStore().ListRelationVersions(ctx,
			store.RelationHistoryQuery{From: "PAGE-2", Type: "references", To: "SPEC-1"})
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond, "default-tail edge should be captured")

	// …and the state-tailed edge's record holds NO versions after
	// further ticks.
	time.Sleep(300 * time.Millisecond)
	var stateVersions int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM relation_versions WHERE rel_record_id = (
		   SELECT rel_record_id FROM relations
		   WHERE from_id = 'PAGE-2' AND from_pointer = 'draft')`).Scan(&stateVersions))
	require.Zero(t, stateVersions, "a state-tailed edge must not mint relation versions in Step 1")
}
