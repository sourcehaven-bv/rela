package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunVersionTests is the conformance suite for content versioning — the
// optional capability bundle a backend advertises via [store.VersionService].
//
// # Why this lives here rather than beside an implementation
//
// pgstore had versioning to itself for a long time, and its guarantees were
// pinned only by tests inside pgstore. That is fine for one implementation and
// actively misleading for two: the second backend's tests describe whatever it
// happens to do, the first backend's describe whatever IT happens to do, and
// nothing anywhere states what a caller may RELY on. The consumers
// (internal/dataentry's history handlers) type-assert the interface and cannot
// see which backend answered, so the contract has to be the thing under test.
//
// Extracted when sqlitestore became the second implementation (TKT-4NU9ZD),
// following the precedent set by Capabilities.TxRollback in TKT-8TJ2WN: the
// backend DECLARES the tier and the suite runs, so a backend that has the
// capability and forgets to say so gets no silent pass.
//
// # What this deliberately does not test
//
// Sweep cadence and debounce windows are mechanism, not contract — a backend
// may capture create/update rows on any schedule, or (like fsstore) not at all.
// The suite therefore drives capture through the SYNCHRONOUS writer, which every
// versioning backend must implement, and asserts on what history READS back.
func RunVersionTests(t *testing.T, f Factory) {
	t.Run("EntityHistory", func(t *testing.T) { runEntityHistoryTests(t, f) })
	t.Run("Faces", func(t *testing.T) { runFaceHistoryTests(t, f) })
	t.Run("Lineage", func(t *testing.T) { runLineageTests(t, f) })
	t.Run("RelationHistory", func(t *testing.T) { runRelationHistoryTests(t, f) })
	t.Run("Purge", func(t *testing.T) { runPurgeTests(t, f) })
}

// versionsOf is the capability handle for a store under test. It fails the test
// rather than skipping: the caller opted in via Capabilities.Versioning, so a
// missing implementation is a broken claim, not an absent feature.
func versionsOf(t *testing.T, s store.Store) store.VersionService {
	t.Helper()
	v, ok := s.(store.VersionServiceProvider)
	require.True(t, ok,
		"store declared Capabilities.Versioning but does not implement store.VersionServiceProvider")
	svc := v.VersionStore()
	require.NotNil(t, svc,
		"VersionStore() returned nil despite the backend declaring Capabilities.Versioning")
	return svc
}

// writeVersion captures one version synchronously, filling the fields every
// backend needs so individual cases stay about the behaviour under test.
func writeVersion(t *testing.T, v store.VersionService, in store.VersionInput) {
	t.Helper()
	if in.SchemaHash == "" {
		in.SchemaHash = "schema-1"
		in.Projection = []byte(`{"v":1}`)
	}
	require.NoError(t, v.WriteVersion(context.Background(), in))
}

func runEntityHistoryTests(t *testing.T, f Factory) {
	t.Run("EmptyHistoryIsNotAnError", func(t *testing.T) {
		v := versionsOf(t, f(t))
		got, err := v.ListVersions(ctx(), "FEAT-404")
		require.NoError(t, err, "an id with no history must read as empty, not error")
		require.Empty(t, got)
	})

	t.Run("VersionsReadBackOldestFirstWithOrdinals", func(t *testing.T) {
		v := versionsOf(t, f(t))
		for _, c := range []string{"first", "second", "third"} {
			writeVersion(t, v, store.VersionInput{
				EntityID: "FEAT-1", Op: store.VersionOpUpdate,
				Type: "feature", Content: c,
			})
		}
		got, err := v.ListVersions(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Len(t, got, 3)
		// Ordinals are 1-based and assigned in lineage order, so a caller can
		// use them as a cursor into this same listing.
		for i, m := range got {
			require.Equal(t, i+1, m.Version)
		}
	})

	t.Run("GetVersionReturnsTheSnapshotContent", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpUpdate, Type: "feature",
			Content: "the old words", Properties: map[string]any{"title": "Old"},
		})
		snap, err := v.GetVersion(ctx(), "FEAT-1", 1)
		require.NoError(t, err)
		require.Equal(t, "the old words", snap.Content)
		require.Equal(t, "Old", snap.Properties["title"])
		require.Equal(t, 1, snap.Version)
	})

	t.Run("OutOfRangeOrdinalIsNotFound", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpUpdate, Type: "feature",
		})
		for _, ord := range []int{0, -1, 2, 99} {
			_, err := v.GetVersion(ctx(), "FEAT-1", ord)
			require.ErrorIs(t, err, store.ErrNotFound,
				"ordinal %d is outside the lineage and must be ErrNotFound", ord)
		}
	})

	t.Run("HistorySurvivesEntityDeletion", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		e := entity.New("FEAT-1", "feature")
		e.SetString("title", "Doomed")
		require.NoError(t, s.CreateEntity(ctx(), e))

		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpDelete, Type: "feature",
			Content: "last words",
		})
		_, err := s.DeleteEntity(ctx(), "FEAT-1", true)
		require.NoError(t, err)

		// The whole point of a compliance history: the row is gone, the record
		// of it is not.
		got, err := v.ListVersions(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, store.VersionOpDelete, got[0].Op)
	})

	t.Run("AttributionRoundTrips", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpDelete, Type: "feature",
			PrincipalUser: "alice", PrincipalTool: "cli", TriggeredBy: "manual",
		})
		got, err := v.ListVersions(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "alice", got[0].PrincipalUser)
		require.Equal(t, "cli", got[0].PrincipalTool)
		require.Equal(t, "manual", got[0].TriggeredBy)
	})
}

func runFaceHistoryTests(t *testing.T, f Factory) {
	t.Run("FacesHaveIndependentLineages", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "", Op: store.VersionOpUpdate,
			Type: "feature", Content: "default content",
		})
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "draft", Op: store.VersionOpUpdate,
			Type: "feature", Content: "draft content",
		})

		// Version 1 of each face is a DIFFERENT snapshot. If faces shared a
		// lineage, one of these would be the other's version 2.
		def, err := v.GetStateVersion(ctx(), "FEAT-1", "", 1)
		require.NoError(t, err)
		require.Equal(t, "default content", def.Content)

		draft, err := v.GetStateVersion(ctx(), "FEAT-1", "draft", 1)
		require.NoError(t, err)
		require.Equal(t, "draft content", draft.Content)

		defList, err := v.ListStateVersions(ctx(), "FEAT-1", "")
		require.NoError(t, err)
		require.Len(t, defList, 1, "the draft face must not appear in the default face's history")
	})

	t.Run("ListVersionsIsTheDefaultFace", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "draft", Op: store.VersionOpUpdate,
			Type: "feature", Content: "draft only",
		})
		got, err := v.ListVersions(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Empty(t, got, "ListVersions must read the DEFAULT face, not any face")
	})

	t.Run("IdenticalContentAcrossFacesStaysDistinct", func(t *testing.T) {
		v := versionsOf(t, f(t))
		const same = "byte identical"
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "", Op: store.VersionOpUpdate,
			Type: "feature", Content: same,
		})
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "published", Op: store.VersionOpUpdate,
			Type: "feature", Content: same,
		})

		// The content hash must fold in the face. If it did not, a dedup
		// keyed on content would silently drop one face's capture — a MISSING
		// version rather than a duplicate, which is the harder bug to notice.
		def, err := v.ListStateVersions(ctx(), "FEAT-1", "")
		require.NoError(t, err)
		require.Len(t, def, 1)
		pub, err := v.ListStateVersions(ctx(), "FEAT-1", "published")
		require.NoError(t, err)
		require.Len(t, pub, 1)
		require.NotEqual(t, def[0].ContentHash, pub[0].ContentHash,
			"two faces holding identical bytes must hash differently")
	})
}

func runLineageTests(t *testing.T, f Factory) {
	t.Run("RenameStitchesHistoryAcrossIDs", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-A", Op: store.VersionOpUpdate, Type: "feature", Content: "as A",
		})
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-B", Op: store.VersionOpRename, PrevID: "FEAT-A",
			Type: "feature", Content: "as B",
		})

		// Reading B must include its life as A: a rename is a continuation, not
		// a new entity.
		got, err := v.ListVersions(ctx(), "FEAT-B")
		require.NoError(t, err)
		require.Len(t, got, 2, "history of B must include its pre-rename life as A")
		require.Equal(t, store.VersionOpUpdate, got[0].Op)
		require.Equal(t, store.VersionOpRename, got[1].Op)
	})

	t.Run("ReusedIDDoesNotMergeUnrelatedHistories", func(t *testing.T) {
		v := versionsOf(t, f(t))
		// An entity lives as A, is renamed to B, and a LATER, unrelated entity
		// is then created on the now-free id A.
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-A", Op: store.VersionOpUpdate, Type: "feature", Content: "original A",
		})
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-B", Op: store.VersionOpRename, PrevID: "FEAT-A",
			Type: "feature", Content: "now B",
		})
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-A", Op: store.VersionOpCreate, Type: "feature", Content: "reused A",
		})

		// This is the fence. A flat `WHERE entity_id = 'FEAT-A'` returns both
		// the original and the reuse, silently merging two unrelated entities
		// into one timeline.
		reused, err := v.ListVersions(ctx(), "FEAT-A")
		require.NoError(t, err)
		require.Len(t, reused, 1,
			"the reused id must see only its own history, not the entity that was renamed away")
		snap, err := v.GetVersion(ctx(), "FEAT-A", 1)
		require.NoError(t, err)
		require.Equal(t, "reused A", snap.Content)

		// And B keeps its full lineage, unaffected by the reuse.
		bHist, err := v.ListVersions(ctx(), "FEAT-B")
		require.NoError(t, err)
		require.Len(t, bHist, 2, "B's lineage must not absorb the reused id's rows")
	})
}

func runRelationHistoryTests(t *testing.T, f Factory) {
	t.Run("EmptyHistoryIsNotAnError", func(t *testing.T) {
		v := versionsOf(t, f(t))
		got, err := v.ListRelationVersions(ctx(), store.RelationHistoryQuery{
			From: "FEAT-1", Type: "rel", To: "FEAT-2",
		})
		require.NoError(t, err, "an unknown key must read as empty, not error")
		require.Empty(t, got)
	})

	t.Run("VersionsReadBackWithOrdinals", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		rid := seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "one", "two")
		require.NotZero(t, rid)

		got, err := v.ListRelationVersions(ctx(), store.RelationHistoryQuery{
			From: "FEAT-1", Type: "rel", To: "FEAT-2",
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for i, m := range got {
			require.Equal(t, i+1, m.Version)
		}
	})

	t.Run("SnapshotContentReadsBack", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "recorded body")

		snap, err := v.GetRelationVersion(ctx(), store.RelationHistoryQuery{
			From: "FEAT-1", Type: "rel", To: "FEAT-2",
		}, 1)
		require.NoError(t, err)
		require.Equal(t, "recorded body", snap.Content)
	})

	t.Run("UnknownLifetimeHandleIsNotFound", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "body")

		// A RecordID that is not a lifetime of this key must not be readable:
		// the composite key is the authorization boundary, so a caller cannot
		// reach an arbitrary lineage by guessing a handle.
		_, err := v.ListRelationVersions(ctx(), store.RelationHistoryQuery{
			From: "FEAT-1", Type: "rel", To: "FEAT-2", RecordID: 999999,
		})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("LifetimesEnumerateForALiveKey", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "body")

		lts, err := v.ListRelationLifetimes(ctx(), "FEAT-1", "rel", "FEAT-2")
		require.NoError(t, err)
		require.Len(t, lts, 1)
		require.Equal(t, 1, lts[0].Lifetime)
		require.True(t, lts[0].Live, "the live relation's lifetime must be flagged Live")
		require.NotZero(t, lts[0].RecordID)
	})

	t.Run("UnknownKeyHasNoLifetimes", func(t *testing.T) {
		v := versionsOf(t, f(t))
		lts, err := v.ListRelationLifetimes(ctx(), "NOPE-1", "rel", "NOPE-2")
		require.NoError(t, err)
		require.Empty(t, lts)
	})
}

// seedRelationLineage creates a live relation and captures one version per
// supplied body, returning the lineage's record id.
//
// It reads the record id back through ListRelationLifetimes rather than
// assuming how a backend allocates one — the surrogate is deliberately opaque.
func seedRelationLineage(
	t *testing.T, s store.Store, v store.VersionService, from, relType, to string, bodies ...string,
) int64 {
	t.Helper()
	for _, id := range []string{from, to} {
		if _, err := s.GetEntity(ctx(), id); err != nil {
			e := entity.New(id, "feature")
			e.SetString("title", id)
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
	}
	_, err := s.CreateRelation(ctx(), from, relType, to, &store.RelationData{})
	require.NoError(t, err)

	// Ask the LIVE ROW for its lineage, not history.
	//
	// ListRelationLifetimes is the wrong source here even though it looks like
	// the caller-facing one: it summarizes version rows, so a relation created
	// a moment ago has no lifetime yet, and after a delete+recreate its newest
	// lifetime is the DEAD one. Seeding the new life's versions onto that id
	// merges two histories that must stay apart — which is what made the
	// multi-lifetime purge case unreachable on pgstore and skip silently.
	rid := relationRecordID(t, s, v, from, relType, to)
	if rid == 0 {
		// The backend exposes no accessor. Fall back to capturing one version
		// and resolving through history, which is correct for a first
		// lifetime — the only case a backend without the accessor can reach.
		require.NoError(t, v.WriteRelationVersion(ctx(), store.RelationVersionInput{
			From: from, Type: relType, To: to, Op: store.VersionOpUpdate,
			Content: bodies[0], SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
		}))
		bodies = bodies[1:]
		lts, ltErr := v.ListRelationLifetimes(ctx(), from, relType, to)
		require.NoError(t, ltErr)
		require.NotEmpty(t, lts)
		rid = lts[0].RecordID
	}
	for _, b := range bodies {
		require.NoError(t, v.WriteRelationVersion(ctx(), store.RelationVersionInput{
			RecordID: rid, From: from, Type: relType, To: to,
			Op: store.VersionOpUpdate, Content: b,
			SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
		}))
	}
	return rid
}

// recordIDer is the optional accessor that answers "which lineage does the
// LIVE row belong to". Backends place it differently — sqlitestore on the
// Store, pgstore on its VersionStore — so the lookup below tries both.
type recordIDer interface {
	RelationRecordID(ctx context.Context, from, relType, to string) (int64, error)
}

// relationRecordID asks the backend for the live row's surrogate lineage id.
//
// This is NOT reachable through ListRelationLifetimes, and the difference is
// the whole reason the accessor exists. A lifetime is summarized from VERSION
// ROWS, so a freshly created relation that has not been captured yet has no
// lifetime at all — and after a delete+recreate, the only lifetime on offer is
// the DEAD one. Seeding through it would append the new life's versions to the
// old life's lineage, merging exactly the two histories rel_record_id exists to
// keep apart.
//
// A backend that exposes it nowhere yields 0, the "unassigned" value every
// implementation understands: the capture still lands, it simply starts its own
// lineage.
func relationRecordID(t *testing.T, s store.Store, v store.VersionService, from, relType, to string) int64 {
	t.Helper()
	for _, candidate := range []any{s, v} {
		r, ok := candidate.(recordIDer)
		if !ok {
			continue
		}
		id, err := r.RelationRecordID(ctx(), from, relType, to)
		if err != nil {
			return 0
		}
		return id
	}
	return 0
}

func runPurgeTests(t *testing.T, f Factory) {
	t.Run("DryRunResolvesWithoutDeleting", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpUpdate, Type: "feature", Content: "secret",
		})
		res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
			EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true}, DryRun: true,
		})
		require.NoError(t, err)
		require.Len(t, res.Targets, 1, "a dry run must still resolve its targets")
		require.Zero(t, res.Purged, "a dry run must delete nothing")

		got, err := v.ListVersions(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Len(t, got, 1, "the version must survive a dry run")
	})

	t.Run("PurgeDeletesTheLineage", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpDelete, Type: "feature", Content: "secret",
		})
		res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
			EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true},
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.Purged)

		got, err := v.ListVersions(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Empty(t, got, "purged history must be gone")
	})

	t.Run("RefusesARenameRow", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-B", Op: store.VersionOpRename, PrevID: "FEAT-A",
			Type: "feature", Content: "renamed",
		})
		res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
			EntityID: "FEAT-B", Selector: store.PurgeSelector{All: true},
		})
		require.NoError(t, err, "a refusal is a result, not an error")
		require.True(t, res.RenameInTargets)
		require.Zero(t, res.Purged, "purging a rename row would orphan the lineage walk")

		got, err := v.ListVersions(ctx(), "FEAT-B")
		require.NoError(t, err)
		require.NotEmpty(t, got, "a refused purge must delete nothing")
	})

	t.Run("RefusesWhileALiveRowHoldsTheContent", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		e := entity.New("FEAT-1", "feature")
		e.SetString("title", "Live")
		require.NoError(t, s.CreateEntity(ctx(), e))

		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpUpdate, Type: "feature",
			Content: e.Content, Properties: e.Properties,
		})
		res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
			EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true},
		})
		require.NoError(t, err)
		require.True(t, res.LiveRowExists)
		require.Zero(t, res.Purged,
			"without ForceLive the sweep would re-capture the content and the erasure would be a lie")
	})

	t.Run("ForceLivePurgesAndTombstones", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		e := entity.New("FEAT-1", "feature")
		e.SetString("title", "Live")
		require.NoError(t, s.CreateEntity(ctx(), e))

		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpUpdate, Type: "feature",
			Content: e.Content, Properties: e.Properties,
		})
		res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
			EntityID: "FEAT-1", Selector: store.PurgeSelector{All: true}, ForceLive: true,
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.Purged)
		require.True(t, res.TombstoneWritten,
			"a ForceLive purge must tombstone, or the sweep re-captures what was just erased")
	})

	t.Run("SelectorIsRequired", func(t *testing.T) {
		v := versionsOf(t, f(t))
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Op: store.VersionOpDelete, Type: "feature",
		})
		_, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{EntityID: "FEAT-1"})
		require.Error(t, err,
			"an empty selector must be refused rather than defaulting to erase everything")
	})

	t.Run("PurgeByContentHashIsScopedToOneFace", func(t *testing.T) {
		v := versionsOf(t, f(t))
		const shared = "identical bytes"
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "", Op: store.VersionOpDelete,
			Type: "feature", Content: shared,
		})
		writeVersion(t, v, store.VersionInput{
			EntityID: "FEAT-1", Face: "draft", Op: store.VersionOpDelete,
			Type: "feature", Content: shared,
		})

		def, err := v.ListStateVersions(ctx(), "FEAT-1", "")
		require.NoError(t, err)
		require.Len(t, def, 1)

		res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
			EntityID: "FEAT-1", Face: "",
			Selector: store.PurgeSelector{ContentHash: def[0].ContentHash},
		})
		require.NoError(t, err)
		require.Equal(t, 1, res.Purged)

		// The sibling face holds the same bytes and must be untouched: purge is
		// scoped to one face, so erasing a sibling's history would destroy
		// records the operator never asked about.
		draft, err := v.ListStateVersions(ctx(), "FEAT-1", "draft")
		require.NoError(t, err)
		require.Len(t, draft, 1, "a content-hash purge must not reach into a sibling face")
	})

	t.Run("RelationMultiLifetimeRequiresASelector", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)

		// Two lifetimes of the same triple: create, capture, delete, recreate.
		rid1 := seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "first life")
		require.NoError(t, v.WriteRelationVersion(ctx(), store.RelationVersionInput{
			RecordID: rid1, From: "FEAT-1", Type: "rel", To: "FEAT-2",
			Op: store.VersionOpDelete, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
		}))
		require.NoError(t, s.DeleteRelation(ctx(), "FEAT-1", "rel", "FEAT-2"))
		rid2 := seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "second life")

		require.NotEqual(t, rid1, rid2,
			"delete+recreate reused the lineage id; the new relation would inherit "+
				"the deleted one's history")

		lts, err := v.ListRelationLifetimes(ctx(), "FEAT-1", "rel", "FEAT-2")
		require.NoError(t, err)
		require.Len(t, lts, 2, "delete+recreate must mint a fresh lifetime, not resurrect the old one")

		res, err := v.PurgeRelationVersions(ctx(), store.RelationVersionPurgeRequest{
			From: "FEAT-1", Type: "rel", To: "FEAT-2",
			Selector: store.PurgeSelector{All: true},
		})
		require.NoError(t, err)
		require.True(t, res.MultiLifetimeRefused,
			"purging one of several lifetimes silently would be a false erasure guarantee")
		require.Zero(t, res.Purged)
		require.Equal(t, 2, res.LifetimeCount)
	})

	t.Run("RelationRecordIDAndAllLifetimesAreExclusive", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		rid := seedRelationLineage(t, s, v, "FEAT-1", "rel", "FEAT-2", "body")

		_, err := v.PurgeRelationVersions(ctx(), store.RelationVersionPurgeRequest{
			From: "FEAT-1", Type: "rel", To: "FEAT-2",
			RecordID: rid, AllLifetimes: true,
			Selector: store.PurgeSelector{All: true},
		})
		require.Error(t, err,
			"a contradictory selector must be refused at the store, which is the trust boundary")
		require.False(t, errors.Is(err, store.ErrNotFound),
			"the refusal should describe the contradiction, not masquerade as a missing record")
	})
}
