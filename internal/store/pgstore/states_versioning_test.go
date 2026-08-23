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

// These tests were written to pin the Step-1 default-world versioning scope
// (TKT-DOFYR1): the sweep's `pointer = ''` / `from_pointer = ''` skips.
// TKT-C1XUA8 removed those skips and gave each face its own lineage, so the
// tests now pin the property the skips were PROTECTING rather than the skips
// themselves — faces must not interleave in one history.
//
// That is the invariant worth keeping either way. Under the old skip it held
// because state rows were never captured; it now holds because
// entity_versions keys (entity_id, pointer, vseq) and every probe is
// face-scoped. A regression in the SQL scoping would show up here as a face's
// rows appearing in a sibling's timeline.

// TestSweep_CapturesEachFaceInItsOwnLineage: both faces are captured, and
// neither appears in the other's history.
func TestSweep_CapturesEachFaceInItsOwnLineage(t *testing.T) {
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

	// The concrete VersionStore already carries the face-aware methods; the
	// assertion is that it satisfies the optional store capability, which is
	// how a consumer reaches them.
	var sh store.StateHistoryReader = s.VersionStore()

	// Both faces are captured, each exactly once.
	require.Eventually(t, func() bool {
		def, e1 := s.VersionStore().ListVersions(ctx, "PAGE-1")
		dr, e2 := sh.ListStateVersions(ctx, "PAGE-1", p)
		return e1 == nil && e2 == nil && len(def) == 1 && len(dr) == 1
	}, 3*time.Second, 25*time.Millisecond, "each face should be captured once")

	// And they STAY separate. A second interval's worth of ticks would
	// re-capture if a face's dedup probe were reading a sibling's hash, and
	// a face's row appearing in the other's timeline is the interleaving the
	// Step-1 skip existed to prevent.
	time.Sleep(300 * time.Millisecond)

	def, err := s.VersionStore().ListVersions(ctx, "PAGE-1")
	require.NoError(t, err)
	require.Len(t, def, 1, "the default face's lineage must hold only its own row")

	dr, err := sh.ListStateVersions(ctx, "PAGE-1", p)
	require.NoError(t, err)
	require.Len(t, dr, 1, "the draft face's lineage must hold only its own row")

	// The snapshots must be the faces' OWN content — the sharpest way to
	// catch a lineage that resolved to the wrong face.
	defSnap, err := s.VersionStore().GetVersion(ctx, "PAGE-1", 1)
	require.NoError(t, err)
	require.Equal(t, "default face", defSnap.Content)

	drSnap, err := sh.GetStateVersion(ctx, "PAGE-1", p, 1)
	require.NoError(t, err)
	require.Equal(t, "draft face", drSnap.Content)
}

// TestSweep_CapturesStateTailedRelations: both tails of the SAME triple mint
// their own relation versions, into distinct rel_record_id lineages that
// could never merge.
//
// The tail is nonetheless recorded on each version row, because the rename
// stitch matches a predecessor by (prev_from, rel_type, prev_to) and without
// the tail could not tell a state-tailed predecessor from a default-tail one
// holding the same triple.
func TestSweep_CapturesStateTailedRelations(t *testing.T) {
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

	// …and so is the state-tailed edge, into its OWN lineage. Each tail has
	// its own rel_record_id, so the two could never merge — what the
	// from_pointer column buys is the rename STITCH being able to tell them
	// apart when it matches a predecessor by triple.
	time.Sleep(300 * time.Millisecond)

	var stateVersions int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM relation_versions WHERE rel_record_id = (
		   SELECT rel_record_id FROM relations
		   WHERE from_id = 'PAGE-2' AND from_pointer = 'draft')`).Scan(&stateVersions))
	require.Equal(t, 1, stateVersions, "a state-tailed edge mints its own relation version")

	// The captured tail is recorded, so the stitch can discriminate.
	var capturedTail string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT from_pointer FROM relation_versions WHERE rel_record_id = (
		   SELECT rel_record_id FROM relations
		   WHERE from_id = 'PAGE-2' AND from_pointer = 'draft')`).Scan(&capturedTail))
	require.Equal(t, "draft", capturedTail,
		"the version must record which tail it captured, or the rename stitch "+
			"cannot tell a state-tailed predecessor from a default-tail one")

	// The default-tail lineage stays at exactly one row: the two faces must
	// not have interleaved.
	metas, err := s.VersionStore().ListRelationVersions(ctx,
		store.RelationHistoryQuery{From: "PAGE-2", Type: "references", To: "SPEC-1"})
	require.NoError(t, err)
	require.Len(t, metas, 1, "the default-tail lineage must hold only its own row")
}

// TestFacesWithIdenticalContentDoNotDedupAgainstEachOther is the copy-vs-sweep
// dedup check the design doc's §13 "verify during implementation" list asks
// for, and the answer is: HARMLESS, but only because two things agree.
//
// The scenario is the copy kernel's ordinary output — promote writes the
// draft's bytes into the published face, so two faces briefly hold IDENTICAL
// content. The sweep then settles and captures both. Does the second capture
// dedup against the first and vanish?
//
// No, for two independent reasons, and the test asserts the outcome rather
// than either mechanism so it survives a refactor of either:
//
//  1. contentHashOf folds the POINTER into the hash (canonical.HashEntity
//     writes it when non-zero), so identical bytes on different faces hash
//     differently.
//  2. The sweep's dedup probes are scoped `ev.pointer = e.pointer`, so a
//     face compares only against its own latest capture.
//
// Either alone would be enough FOR THIS SCENARIO, and both are present
// because each guards a failure the other does not:
//
//   - Drop (1) and this test fails — and, worse, a purge ForceLive tombstone
//     on one face carries a hash matching a sibling holding identical bytes,
//     suppressing that sibling's legitimate capture.
//   - Drop (2) and this test still PASSES, because the hash already
//     discriminates. What (2) additionally guards is the delete fence: the
//     `lvc` probe's since-last-delete subselect must be face-scoped, or one
//     face's delete resets another face's lifecycle boundary. That is a
//     different scenario, covered by the delete/recreate tests.
//
// Verified by removing each mechanism in turn. The failure mode throughout is
// a MISSING version, not a duplicate — silent, and invisible until someone
// opens a history that should not be empty.
func TestFacesWithIdenticalContentDoNotDedupAgainstEachOther(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// Byte-identical faces — exactly what a promote leaves behind.
	const shared = "identical on both faces"
	require.NoError(t, s.CreateEntity(ctx, mkEntity("PAGE-9", "ticket", shared)))
	published := mkEntity("PAGE-9", "ticket", shared)
	p, err := entity.ParsePointer("published")
	require.NoError(t, err)
	published.Pointer = p
	require.NoError(t, s.CreateEntity(ctx, published))

	_, err = pool.Exec(ctx,
		`UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = 'PAGE-9'`)
	require.NoError(t, err)

	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	var sh store.StateHistoryReader = s.VersionStore()
	require.Eventually(t, func() bool {
		def, e1 := s.VersionStore().ListVersions(ctx, "PAGE-9")
		pub, e2 := sh.ListStateVersions(ctx, "PAGE-9", p)
		return e1 == nil && e2 == nil && len(def) == 1 && len(pub) == 1
	}, 3*time.Second, 25*time.Millisecond,
		"both faces must be captured despite holding identical content — a face "+
			"deduping against its SIBLING's hash loses a version silently")

	// And the hashes differ, which is the structural half of the guarantee.
	def, err := s.VersionStore().ListVersions(ctx, "PAGE-9")
	require.NoError(t, err)
	pub, err := sh.ListStateVersions(ctx, "PAGE-9", p)
	require.NoError(t, err)
	require.NotEqual(t, def[0].ContentHash, pub[0].ContentHash,
		"the pointer must participate in the content hash, or a purge tombstone "+
			"on one face would suppress a sibling face's legitimate capture")
}
