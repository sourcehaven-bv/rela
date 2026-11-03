package storetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// tagIndexOf returns s's version tag index, failing when a tagging backend
// lacks one.
func tagIndexOf(t *testing.T, s store.Store) store.VersionTagIndex {
	t.Helper()
	ix, ok := versionsOf(t, s).(store.VersionTagIndex)
	require.True(t, ok, "a tagging VersionService must be a store.VersionTagIndex")
	return ix
}

// taggedEntities lists the entities tagged name, of entityType when set, and
// checks every row against the Go lineage reads.
func taggedEntities(t *testing.T, s store.Store, name, entityType string) []store.TaggedEntity {
	t.Helper()
	got, err := tagIndexOf(t, s).TaggedEntities(ctx(), mustTagName(t, name), entityType)
	require.NoError(t, err)
	requireTaggedConsistent(t, s, name, got)
	return got
}

// tagged is the expected row for ref.
func tagged(ref entity.Ref, typ string, version int, deleted bool) store.TaggedEntity {
	return store.TaggedEntity{Ref: ref, Type: typ, Version: version, Deleted: deleted}
}

// requireVersionByTag asserts the live row's Version is the ordinal
// VersionByTag serves for the tag "base".
func requireVersionByTag(t *testing.T, tg store.VersionTagger, row store.TaggedEntity) {
	t.Helper()
	snap, err := tg.VersionByTag(ctx(), row.Ref, mustTagName(t, "base"))
	require.NoError(t, err)
	require.Equal(t, snap.Version, row.Version, "TaggedEntities and VersionByTag disagree on %v", row.Ref)
}

// runTaggedEntitiesTests pins [store.VersionTagIndex] (TKT-7LRUPB): every
// lifecycle a tag can end up in is listed once, live or deleted, under the
// face it lives on now, with the ordinal VersionByTag serves.
func (h *tagHarness) runTaggedEntitiesTests(t *testing.T) {
	f1 := entity.Ref{ID: "FEAT-1"}

	t.Run("ListsTypeFiltersAndOrdinals", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		_, err := tg.TagVersion(ctx(), tagAt(t, f1, "base", 2))
		require.NoError(t, err)
		req := entity.New("REQ-1", "requirement")
		require.NoError(t, s.CreateEntity(ctx(), req))
		_, err = tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "REQ-1"}, "base"))
		require.NoError(t, err)
		_, err = tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "REQ-1"}, "other"))
		require.NoError(t, err)

		got := taggedEntities(t, s, "base", "")
		require.Equal(t, []store.TaggedEntity{
			tagged(f1, "feature", 2, false),
			tagged(entity.Ref{ID: "REQ-1"}, "requirement", 1, false),
		}, got)
		requireVersionByTag(t, tg, got[0])
		require.Equal(t, []store.TaggedEntity{tagged(f1, "feature", 2, false)},
			taggedEntities(t, s, "base", "feature"))
		require.Empty(t, taggedEntities(t, s, "base", "nope"))
		require.Empty(t, taggedEntities(t, s, "absent", ""))
	})

	t.Run("ZeroNameIsInvalid", func(t *testing.T) {
		_, err := tagIndexOf(t, h.f(t)).TaggedEntities(ctx(), store.VersionTagName{}, "")
		require.ErrorIs(t, err, store.ErrInvalidVersionTag)
	})

	t.Run("RenameFollowsToTheNewID", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "first")))
		h.sweepNow(t, s)
		require.NoError(t, s.UpdateEntity(ctx(), feature("FEAT-A", "second")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		renameAToBWithVersion(t, s, "second")
		require.NoError(t, s.UpdateEntity(ctx(), feature("FEAT-B", "third")))
		h.sweepNow(t, s)

		got := taggedEntities(t, s, "base", "")
		require.Equal(t, []store.TaggedEntity{tagged(entity.Ref{ID: "FEAT-B"}, "feature", 2, false)}, got)
		requireVersionByTag(t, tg, got[0])
	})

	t.Run("DeletedIsListed", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		_, err := tg.TagVersion(ctx(), tagAt(t, f1, "base", 1))
		require.NoError(t, err)
		deleteWithVersion(t, s, "FEAT-1")
		require.Equal(t, []store.TaggedEntity{tagged(f1, "feature", 1, true)}, taggedEntities(t, s, "base", ""))
		require.Equal(t, []store.TaggedEntity{tagged(f1, "feature", 1, true)},
			taggedEntities(t, s, "base", "feature"), "a deleted row filters on the tagged version's type")

		// A new entity on the id does not revive the old lifecycle's row.
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "new")))
		h.sweepNow(t, s)
		require.Equal(t, []store.TaggedEntity{tagged(f1, "feature", 1, true)}, taggedEntities(t, s, "base", ""))
	})

	t.Run("DeleteAfterRenameIsListedUnderTheLastID", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		renameAToBWithVersion(t, s, "a")
		deleteWithVersion(t, s, "FEAT-B")
		require.Equal(t, []store.TaggedEntity{tagged(entity.Ref{ID: "FEAT-B"}, "feature", 1, true)},
			taggedEntities(t, s, "base", ""))
	})

	t.Run("RenameOntoADeletedIDListsBoth", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-B", "old b")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-B"}, "base"))
		require.NoError(t, err)
		deleteWithVersion(t, s, "FEAT-B")
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err = tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		renameAToBWithVersion(t, s, "a")

		got := taggedEntities(t, s, "base", "")
		require.Len(t, got, 2)
		byDeleted := map[bool]store.TaggedEntity{got[0].Deleted: got[0], got[1].Deleted: got[1]}
		require.Equal(t, tagged(entity.Ref{ID: "FEAT-B"}, "feature", 1, true), byDeleted[true])
		require.Equal(t, entity.Ref{ID: "FEAT-B"}, byDeleted[false].Ref)
		requireVersionByTag(t, tg, byDeleted[false])
	})

	t.Run("RenameOntoAnIDDeletedAfterTheTagStaysLive", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-B", "old b")))
		h.sweepNow(t, s)
		deleteWithVersion(t, s, "FEAT-B")
		renameAToBWithVersion(t, s, "a")

		got := taggedEntities(t, s, "base", "")
		require.Equal(t, []store.TaggedEntity{tagged(entity.Ref{ID: "FEAT-B"}, "feature", 1, false)}, got)
		requireVersionByTag(t, tg, got[0])
	})

	// The deleted id is taken by a new entity that is then renamed away, so
	// FEAT-A's listed history starts after the tagged row: no ordinal.
	t.Run("ReusedIDRenamedAwayHasNoOrdinal", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "old a")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		deleteWithVersion(t, s, "FEAT-A")
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "new a")))
		h.sweepNow(t, s)
		renameAToBWithVersion(t, s, "new a")

		require.Equal(t, []store.TaggedEntity{tagged(entity.Ref{ID: "FEAT-A"}, "feature", 0, true)},
			taggedEntities(t, s, "base", ""))
	})

	t.Run("PerFace", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		draft := entity.Ref{ID: "FEAT-2", Face: "draft"}
		published := entity.Ref{ID: "FEAT-2", Face: "published"}
		require.NoError(t, s.CreateEntity(ctx(), faced("FEAT-2", "draft", "d")))
		require.NoError(t, s.CreateEntity(ctx(), faced("FEAT-2", "published", "p")))
		h.sweepNow(t, s)
		_, err := tg.TagCurrent(ctx(), tagReq(t, draft, "base"))
		require.NoError(t, err)
		_, err = tg.TagCurrent(ctx(), tagReq(t, published, "base"))
		require.NoError(t, err)

		_, err = s.DeleteFace(ctx(), draft)
		require.NoError(t, err)
		writeVersion(t, versionsOf(t, s), store.VersionInput{
			EntityID: "FEAT-2", Face: "draft", Op: store.VersionOpDelete, Type: "feature",
			PrincipalUser: "alice", PrincipalTool: "test",
		})
		require.Equal(t, []store.TaggedEntity{
			tagged(draft, "feature", 1, true),
			tagged(published, "feature", 1, false),
		}, taggedEntities(t, s, "base", ""))
	})

	t.Run("SoftDeletedIsDeleted", func(t *testing.T) {
		s := h.f(t)
		sd, ok := s.(store.SoftDeleteProvider)
		if !ok {
			t.Skip("backend has no soft delete")
		}
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "body")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, f1, "base"))
		require.NoError(t, err)
		_, err = sd.SoftDelete().MarkDeleted(ctx(), "FEAT-1", "alice")
		require.NoError(t, err)
		require.Equal(t, []store.TaggedEntity{tagged(f1, "feature", 1, true)}, taggedEntities(t, s, "base", ""))
		_, err = sd.SoftDelete().Unmark(ctx(), "FEAT-1")
		require.NoError(t, err)
		require.Equal(t, []store.TaggedEntity{tagged(f1, "feature", 1, false)}, taggedEntities(t, s, "base", ""))
	})

	h.runTaggedLineageTests(t)
}
