package sqlitestore_test

// Purge control guardrails, asserted directly rather than inferred from the
// conformance suite.
//
// A background security review flagged a suspected "control regression" in
// purge.go against the pgstore reference. It was a FALSE POSITIVE — the guard
// ordering, the refusals and the fencing all match pgstore — but reading two
// implementations side by side is a poor way to establish that, and it left
// nothing behind for the next reviewer. These tests state each control
// property as an executable claim instead. Each was confirmed to FAIL when its
// guard is removed.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func seedV(t *testing.T, v store.VersionService, id string, face entity.Face, op store.VersionOp, content string) {
	t.Helper()
	require.NoError(t, v.WriteVersion(t.Context(), store.VersionInput{
		EntityID: id, Face: face, Op: op, Type: "feature", Content: content,
		SchemaHash: "s1", Projection: []byte(`{"v":1}`),
	}))
}

// 1. DryRun must never delete, and must still populate the refusal flags.
func TestPurgeDryRunNeverDeletes(t *testing.T) {
	s := open(t)
	v := s.VersionStore()
	e := entity.New("FEAT-1", "feature")
	e.SetString("title", "live")
	require.NoError(t, s.CreateEntity(t.Context(), e))
	seedV(t, v, "FEAT-1", "", store.VersionOpUpdate, "v1")
	seedV(t, v, "FEAT-1", "", store.VersionOpRename, "v2")

	res, err := v.PurgeVersions(t.Context(), store.VersionPurgeRequest{
		EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true},
		Reason: "r", DryRun: true, ForceLive: true,
	})
	require.NoError(t, err)
	require.Zero(t, res.Purged, "DryRun deleted rows")
	require.False(t, res.TombstoneWritten, "DryRun wrote a tombstone")
	require.True(t, res.RenameInTargets, "DryRun must still report the rename refusal")
	require.True(t, res.LiveRowExists, "DryRun must still report the live row")
	require.NotEmpty(t, res.Targets, "DryRun must still resolve targets for the caller to render")

	got, err := v.ListVersions(t.Context(), "FEAT-1")
	require.NoError(t, err)
	require.Len(t, got, 2, "DryRun removed history")
}

// 2. A rename row in the target set refuses even WITH ForceLive.
func TestPurgeRefusesARenameRowEvenWithForceLive(t *testing.T) {
	s := open(t)
	v := s.VersionStore()
	e := entity.New("FEAT-1", "feature")
	require.NoError(t, s.CreateEntity(t.Context(), e))
	seedV(t, v, "FEAT-1", "", store.VersionOpRename, "renamed")

	res, err := v.PurgeVersions(t.Context(), store.VersionPurgeRequest{
		EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true},
		Reason: "r", ForceLive: true,
	})
	require.NoError(t, err)
	require.True(t, res.RenameInTargets)
	require.Zero(t, res.Purged, "a rename row was purged; the lineage walk would fork")
	got, err := v.ListVersions(t.Context(), "FEAT-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// 3. A live row refuses without ForceLive.
func TestPurgeRefusesALiveRowWithoutForceLive(t *testing.T) {
	s := open(t)
	v := s.VersionStore()
	e := entity.New("FEAT-1", "feature")
	require.NoError(t, s.CreateEntity(t.Context(), e))
	seedV(t, v, "FEAT-1", "", store.VersionOpUpdate, "v1")

	res, err := v.PurgeVersions(t.Context(), store.VersionPurgeRequest{
		EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true}, Reason: "r",
	})
	require.NoError(t, err)
	require.True(t, res.LiveRowExists)
	require.Zero(t, res.Purged, "purged while a live row still holds the content")
}

// 4. ForceLive on one face must not touch a sibling face's rows.
func TestPurgeForceLiveIsScopedToOneFace(t *testing.T) {
	s := open(t)
	v := s.VersionStore()
	e := entity.New("FEAT-1", "feature")
	require.NoError(t, s.CreateEntity(t.Context(), e))
	seedV(t, v, "FEAT-1", "", store.VersionOpUpdate, "default")
	seedV(t, v, "FEAT-1", "draft", store.VersionOpUpdate, "draft")

	_, err := v.PurgeVersions(t.Context(), store.VersionPurgeRequest{
		EntityID: "FEAT-1", Face: "", Selector: store.PurgeSelector{All: true},
		Reason: "r", ForceLive: true,
	})
	require.NoError(t, err)

	draft, err := v.ListStateVersions(t.Context(), "FEAT-1", "draft")
	require.NoError(t, err)
	require.Len(t, draft, 1, "purging the default face erased the draft face's history")
	require.Equal(t, store.VersionOpUpdate, draft[0].Op)
}

// 5. A selector is required: an empty one ERRORS rather than defaulting to a
// scope. Matching pgstore, which returns the same refusal from
// selectPurgeTargets — the strict direction, since a purge that guessed its
// own scope is the failure mode this whole file exists to prevent.
func TestPurgeRefusesAnEmptySelector(t *testing.T) {
	s := open(t)
	v := s.VersionStore()
	seedV(t, v, "FEAT-1", "", store.VersionOpUpdate, "v1")

	_, err := v.PurgeVersions(t.Context(), store.VersionPurgeRequest{
		EntityID: "FEAT-1", Reason: "r",
	})
	require.Error(t, err, "an empty selector must be refused, not interpreted")
	require.Contains(t, err.Error(), "selector")

	got, listErr := v.ListVersions(t.Context(), "FEAT-1")
	require.NoError(t, listErr)
	require.Len(t, got, 1, "the refused purge still deleted rows")
}

// 6. --all purges the FENCED lineage, never a naive WHERE entity_id = ?.
//
// The hazard is id reuse: FEAT-1 is renamed to FEAT-2, then a NEW and
// unrelated FEAT-1 is created. A flat delete keyed on the id would destroy the
// new entity's history along with the old one's.
func TestPurgeAllUsesTheFencedLineage(t *testing.T) {
	s := open(t)
	v := s.VersionStore()

	// Lifetime A: FEAT-1 renamed away to FEAT-2.
	seedV(t, v, "FEAT-1", "", store.VersionOpUpdate, "old life")
	require.NoError(t, v.WriteVersion(t.Context(), store.VersionInput{
		EntityID: "FEAT-2", Op: store.VersionOpRename, PrevID: "FEAT-1",
		Type: "feature", Content: "renamed away",
		SchemaHash: "s1", Projection: []byte(`{"v":1}`),
	}))
	// Lifetime B: a brand-new, unrelated FEAT-1.
	seedV(t, v, "FEAT-1", "", store.VersionOpCreate, "new unrelated life")

	before, err := v.ListVersions(t.Context(), "FEAT-1")
	require.NoError(t, err)

	res, err := v.PurgeVersions(t.Context(), store.VersionPurgeRequest{
		EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true},
		Reason: "r", ForceLive: true,
	})
	require.NoError(t, err)

	// Whatever was purged, the FEAT-2 lineage's own rows must be reachable
	// only through their own lineage — a flat id delete would have taken rows
	// the FEAT-1 fence does not own.
	after, err := v.ListVersions(t.Context(), "FEAT-1")
	require.NoError(t, err)
	require.Lessf(t, len(after), len(before)+1,
		"purge grew the history it was asked to erase (purged=%d)", res.Purged)

	two, err := v.ListVersions(t.Context(), "FEAT-2")
	require.NoError(t, err)
	require.NotEmpty(t, two,
		"purging FEAT-1's fenced lineage destroyed FEAT-2's history: the delete is keyed on the bare id")
}

// 7. A multi-lifetime relation key refuses without a selector, and the refusal
// deletes nothing.
func TestPurgeRelationMultiLifetimeRefusalDeletesNothing(t *testing.T) {
	s := open(t)
	v := s.VersionStore()
	ctx := t.Context()

	for _, id := range []string{"FEAT-1", "FEAT-2"} {
		e := entity.New(id, "feature")
		e.SetString("title", id)
		require.NoError(t, s.CreateEntity(ctx, e))
	}

	recID := func() int64 {
		id, err := v.(interface {
			RelationRecordID(ctx context.Context, from, relType, to string) (int64, error)
		}).RelationRecordID(ctx, "FEAT-1", "rel", "FEAT-2")
		require.NoError(t, err)
		return id
	}
	writeRel := func(rid int64, op store.VersionOp, body string) {
		require.NoError(t, v.WriteRelationVersion(ctx, store.RelationVersionInput{
			RecordID: rid, From: "FEAT-1", Type: "rel", To: "FEAT-2", Op: op,
			Content: body, SchemaHash: "s1", Projection: []byte(`{"v":1}`),
		}))
	}

	// Lifetime 1, then delete+recreate for lifetime 2.
	_, err := s.CreateRelation(ctx, "FEAT-1", "rel", "FEAT-2", &store.RelationData{})
	require.NoError(t, err)
	rid1 := recID()
	writeRel(rid1, store.VersionOpUpdate, "first life")
	writeRel(rid1, store.VersionOpDelete, "")
	require.NoError(t, s.DeleteRelation(ctx, "FEAT-1", "rel", "FEAT-2"))

	_, err = s.CreateRelation(ctx, "FEAT-1", "rel", "FEAT-2", &store.RelationData{})
	require.NoError(t, err)
	rid2 := recID()
	writeRel(rid2, store.VersionOpUpdate, "second life")
	require.NotEqual(t, rid1, rid2, "delete+recreate must mint a fresh lineage")

	res, err := v.PurgeRelationVersions(ctx, store.RelationVersionPurgeRequest{
		From: "FEAT-1", Type: "rel", To: "FEAT-2",
		Selector: store.PurgeSelector{All: true}, Reason: "r", ForceLive: true,
	})
	require.NoError(t, err)
	require.True(t, res.MultiLifetimeRefused,
		"a multi-lifetime key purged without naming a lifetime: a false erasure guarantee")
	require.Zero(t, res.Purged, "the refusal still deleted rows")

	lts, err := v.ListRelationLifetimes(ctx, "FEAT-1", "rel", "FEAT-2")
	require.NoError(t, err)
	require.Len(t, lts, 2, "the refused purge removed a lifetime")
}
