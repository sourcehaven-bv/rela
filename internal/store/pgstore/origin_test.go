package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// copyOrigin is the provenance a copy definition would stamp: this test drives
// the STORE directly (it is a pgstore test, not an entitymanager one), so it
// supplies the ctx value the entitymanager write boundary supplies in
// production. The route is identical — store.WithOrigin on ctx — which is the
// point: there is no second way in.
var copyOrigin = store.Origin{
	Kind:       store.OriginCopy,
	Source:     "POL-1",
	SourceFace: "draft",
	SourceType: "policy",
	Definition: "publish",
}

// TestSweep_CapturedVersionCarriesCopyOrigin is the end-to-end check the whole
// feature exists for: a write marked as a copy must produce a version a reader
// can identify AS a copy, naming its source — and an ordinary write must
// produce one that is distinguishable from it.
//
// It runs both cases in ONE sweep over two entities rather than two tests,
// because "distinguishable" is a claim about the PAIR: a test that only
// asserted the copy row would pass just as well if the sweep stamped every
// version with the same origin.
func TestSweep_CapturedVersionCarriesCopyOrigin(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// COPIED: written under a ctx carrying copy provenance.
	copyCtx := store.WithOrigin(
		store.WithAttribution(ctx, store.Attribution{User: "edith@example.com", Tool: "data-entry"}),
		copyOrigin)
	require.NoError(t, s.CreateEntity(copyCtx, mkEntity("POL-2", "policy", "copied body")))

	// HAND-EDITED: same principal, same tool, NO origin. The attribution is
	// deliberately identical so the origin is the only thing that differs —
	// otherwise the test could pass on the principal alone.
	handCtx := store.WithAttribution(ctx,
		store.Attribution{User: "edith@example.com", Tool: "data-entry"})
	require.NoError(t, s.CreateEntity(handCtx, mkEntity("POL-3", "policy", "typed body")))

	startFastSweep(t, s)

	require.Eventually(t, func() bool {
		a, e1 := s.VersionStore().ListVersions(ctx, "POL-2")
		b, e2 := s.VersionStore().ListVersions(ctx, "POL-3")
		return e1 == nil && e2 == nil && len(a) == 1 && len(b) == 1
	}, 5*time.Second, 25*time.Millisecond, "both entities should be captured once")

	copied, err := s.VersionStore().ListVersions(ctx, "POL-2")
	require.NoError(t, err)
	require.Equal(t, copyOrigin, copied[0].Origin,
		"a copy's version must carry the mechanism AND its source; without this "+
			"the copy is indistinguishable from someone typing the same bytes")
	require.Equal(t, "POL-1@draft", copied[0].Origin.SourceLabel())
	// The op is unchanged: a copy is still a create of the target row. This is
	// the compatibility guarantee for every client switching on VersionOp.
	require.Equal(t, store.VersionOpCreate, copied[0].Op)

	hand, err := s.VersionStore().ListVersions(ctx, "POL-3")
	require.NoError(t, err)
	require.True(t, hand[0].Origin.IsZero(),
		"a hand edit must carry NO origin — the absence is the marking, and the "+
			"principal on the same row is what names the editor")
	require.Equal(t, "edith@example.com", hand[0].PrincipalUser)
	require.Equal(t, "data-entry", hand[0].PrincipalTool)

	// The full snapshot read must agree with the timeline read — they are two
	// different queries and either could drift on its own.
	snap, err := s.VersionStore().GetVersion(ctx, "POL-2", 1)
	require.NoError(t, err)
	require.Equal(t, copyOrigin, snap.Origin)
}

// TestOriginIsClearedByASubsequentHandEdit pins the semantics that make the
// marker mean anything: the columns describe the MOST RECENT write, so editing
// a copied row by hand must stop the row claiming to be a copy.
//
// Without this, origin would be sticky and every later version of a
// once-copied entity would falsely claim copy provenance — the row's distant
// past masquerading as this version's.
func TestOriginIsClearedByASubsequentHandEdit(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	copyCtx := store.WithOrigin(ctx, copyOrigin)
	require.NoError(t, s.CreateEntity(copyCtx, mkEntity("POL-4", "policy", "copied body")))
	startFastSweep(t, s)

	require.Eventually(t, func() bool {
		v, e := s.VersionStore().ListVersions(ctx, "POL-4")
		return e == nil && len(v) == 1
	}, 5*time.Second, 25*time.Millisecond)

	// A plain edit — no origin on ctx at all.
	require.NoError(t, s.UpdateEntity(ctx, mkEntity("POL-4", "policy", "hand-edited body")))

	require.Eventually(t, func() bool {
		v, e := s.VersionStore().ListVersions(ctx, "POL-4")
		return e == nil && len(v) == 2
	}, 5*time.Second, 25*time.Millisecond, "the hand edit should be captured too")

	versions, err := s.VersionStore().ListVersions(ctx, "POL-4")
	require.NoError(t, err)
	require.Equal(t, copyOrigin, versions[0].Origin, "v1 was the copy")
	require.True(t, versions[1].Origin.IsZero(),
		"v2 was typed by hand and must not inherit v1's provenance")
}

// TestSyncCaptureCarriesOrigin covers the OTHER capture path. create/update
// arrive via the sweep (above); a synchronous capture carries provenance
// inside store.VersionInput instead, and the two must agree — a field wired
// through only one path is worse than one wired through neither, because the
// timeline silently changes meaning depending on which op produced the row.
func TestSyncCaptureCarriesOrigin(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.VersionStore().WriteVersion(ctx, store.VersionInput{
		EntityID:   "POL-5",
		Op:         store.VersionOpDelete,
		Type:       "policy",
		Content:    "final body",
		SchemaHash: "schema-abc",
		Projection: []byte(`{"entities":{},"types":{}}`),
		Origin:     copyOrigin,
	}))

	versions, err := s.VersionStore().ListVersions(ctx, "POL-5")
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Equal(t, copyOrigin, versions[0].Origin)
}

// startFastSweep starts a sweep tuned to fire within a test's patience.
func startFastSweep(t *testing.T, s *pgstore.Store) {
	t.Helper()
	s.StartVersionSweep(
		stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{
			Interval:     50 * time.Millisecond,
			Idle:         time.Millisecond,
			MaxStaleness: time.Hour,
			Batch:        100,
		})
}
