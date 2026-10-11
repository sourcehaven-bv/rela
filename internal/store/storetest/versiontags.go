package storetest

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// tagProjection is the fixed render projection the tag suite hands the
// tagger, matching what writeVersion stamps.
type tagProjection struct{}

func (tagProjection) Projection() (string, []byte) { return "schema-1", []byte(`{"v":1}`) }

// taggerOf returns s's version tagger, failing when the backend declared
// Capabilities.VersionTags without providing one.
func taggerOf(t *testing.T, s store.Store) store.VersionTagger {
	t.Helper()
	p, ok := versionsOf(t, s).(store.VersionTaggerProvider)
	require.True(t, ok,
		"store declared Capabilities.VersionTags but its VersionService is not a store.VersionTaggerProvider")
	tg, err := p.VersionTagger(tagProjection{})
	require.NoError(t, err)
	return tg
}

// mustTagName parses a tag name the test knows is valid.
func mustTagName(t *testing.T, s string) store.VersionTagName {
	t.Helper()
	n, err := store.ParseVersionTagName(s)
	require.NoError(t, err)
	return n
}

// RunVersionTagTests pins the [store.VersionTagger] contract (TKT-VO6VG9):
// capture-now, moving and removing a tag, which lifecycle a tag belongs to
// across rename, delete and re-creation, per-face scope, the purge refusal,
// the compare-and-set precondition, the in-transaction refusal and
// concurrent moves.
//
// sweepNow must run one sweep tick that treats every row as settled.
// Deletes and renames are captured synchronously above the store, so the
// suite writes those version rows itself, as entitymanager would.
func RunVersionTagTests(t *testing.T, f Factory, sweepNow func(t *testing.T, s store.Store)) {
	h := &tagHarness{f: f, sweepNow: sweepNow}
	t.Run("Names", runTagNameTests)
	t.Run("CaptureNow", h.runCaptureNowTests)
	t.Run("MoveAndRemove", h.runMoveTests)
	t.Run("Lifecycle", h.runLifecycleTests)
	t.Run("Guards", h.runGuardTests)
	t.Run("Purge", h.runPurgeTagTests)
}

type tagHarness struct {
	f        Factory
	sweepNow func(t *testing.T, s store.Store)
}

func feature(id, body string) *entity.Entity {
	e := entity.New(id, "feature")
	e.Content = body
	return e
}

func faced(id string, face entity.Face, body string) *entity.Entity {
	e := feature(id, body)
	e.Face = face
	return e
}

// tagReq is a TagCurrent request by the test principal.
func tagReq(t *testing.T, ref entity.Ref, name string) store.TagRequest {
	t.Helper()
	return store.TagRequest{Ref: ref, Name: mustTagName(t, name), PrincipalUser: "sync", PrincipalTool: "connector"}
}

// tagAt is a TagVersion request by the test principal.
func tagAt(t *testing.T, ref entity.Ref, name string, version int) store.TagRequest {
	t.Helper()
	r := tagReq(t, ref, name)
	r.Version = version
	return r
}

func untagReq(t *testing.T, ref entity.Ref, name string) store.UntagRequest {
	t.Helper()
	return store.UntagRequest{Ref: ref, Name: mustTagName(t, name), PrincipalUser: "sync", PrincipalTool: "connector"}
}

// requireTagAt asserts name resolves to version on ref.
func requireTagAt(t *testing.T, tg store.VersionTagger, ref entity.Ref, name string, version int) {
	t.Helper()
	got, err := tg.VersionByTag(ctx(), ref, mustTagName(t, name))
	require.NoError(t, err, "tag %q on %v", name, ref)
	require.Equal(t, version, got.Version, "tag %q on %v", name, ref)
	require.Contains(t, got.Tags, name, "tag %q on %v", name, ref)
}

// requireNoTag asserts name does not resolve on ref.
func requireNoTag(t *testing.T, tg store.VersionTagger, ref entity.Ref, name string) {
	t.Helper()
	_, err := tg.VersionByTag(ctx(), ref, mustTagName(t, name))
	require.ErrorIs(t, err, store.ErrNotFound, "tag %q on %v", name, ref)
}

// tagsByVersion returns ListVersions' tags keyed by ordinal.
func tagsByVersion(t *testing.T, s store.Store, ref entity.Ref) map[int][]string {
	t.Helper()
	metas, err := versionsOf(t, s).ListVersions(ctx(), ref)
	require.NoError(t, err)
	out := map[int][]string{}
	for _, m := range metas {
		if len(m.Tags) > 0 {
			out[m.Version] = m.Tags
		}
	}
	return out
}

// twoVersions creates FEAT-1, sweeps, updates it and sweeps again.
func (h *tagHarness) twoVersions(t *testing.T, s store.Store) {
	t.Helper()
	require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "first")))
	h.sweepNow(t, s)
	require.NoError(t, s.UpdateEntity(ctx(), feature("FEAT-1", "second")))
	h.sweepNow(t, s)
}

// deleteWithVersion deletes id and writes the delete version entitymanager
// captures synchronously.
func deleteWithVersion(t *testing.T, s store.Store, id string) {
	t.Helper()
	_, err := s.DeleteFamily(ctx(), id, true)
	require.NoError(t, err)
	writeVersion(t, versionsOf(t, s), store.VersionInput{
		EntityID: id, Op: store.VersionOpDelete, Type: "feature",
		PrincipalUser: "alice", PrincipalTool: "test",
	})
}

// renameAToBWithVersion renames FEAT-A to FEAT-B and writes the rename
// version.
func renameAToBWithVersion(t *testing.T, s store.Store, body string) {
	t.Helper()
	const oldID, newID = "FEAT-A", "FEAT-B"
	_, err := s.RenameFamily(ctx(), oldID, newID)
	require.NoError(t, err)
	writeVersion(t, versionsOf(t, s), store.VersionInput{
		EntityID: newID, Op: store.VersionOpRename, PrevID: oldID, Type: "feature", Content: body,
		PrincipalUser: "alice", PrincipalTool: "test",
	})
}

// AC8: tag names.
func runTagNameTests(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		ok   bool
	}{
		{"Plain", "base", true},
		{"Namespaced", "sync/base-1.2_x", true},
		{"Digits", "2026", true},
		{"MaxSegment", strings.Repeat("a", 63), true},
		{"Empty", "", false},
		{"Upper", "Base", false},
		{"TwoSlashes", "a/b/c", false},
		{"EmptyNamespace", "/a", false},
		{"EmptyLocal", "a/", false},
		{"LeadingPunct", "-a", false},
		{"Space", "a b", false},
		{"SegmentTooLong", strings.Repeat("a", 64), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := store.ParseVersionTagName(tc.in)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.in, n.String())
				return
			}
			require.ErrorIs(t, err, store.ErrInvalidVersionTag)
			require.True(t, n.IsZero())
		})
	}
}

// AC1 and the attribution copy.
func (h *tagHarness) runCaptureNowTests(t *testing.T) {
	t.Run("CapturesUnsweptStateAsTheSweepWould", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		origin := store.Origin{Kind: store.OriginCopy, Source: "FEAT-0", SourceType: "feature", Definition: "clone"}
		wctx := store.WithOrigin(store.WithAttribution(ctx(), store.Attribution{User: "alice", Tool: "mcp"}), origin)
		require.NoError(t, s.CreateEntity(wctx, feature("FEAT-1", "body")))

		meta, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-1"}, "sync/base"))
		require.NoError(t, err)
		require.Equal(t, 1, meta.Version)
		require.Equal(t, store.VersionOpCreate, meta.Op)
		require.Equal(t, []string{"sync/base"}, meta.Tags)
		// The capture credits the editor of the bytes, not the tagger.
		require.Equal(t, "alice", meta.PrincipalUser)
		require.Equal(t, "mcp", meta.PrincipalTool)
		require.Equal(t, origin, meta.Origin)

		// The sweep dedups against the captured row rather than adding one.
		h.sweepNow(t, s)
		metas, err := versionsOf(t, s).ListVersions(ctx(), entity.Ref{ID: "FEAT-1"})
		require.NoError(t, err)
		require.Len(t, metas, 1, "the sweep must not re-capture a tagged capture")
		require.Equal(t, []string{"sync/base"}, metas[0].Tags)
		requireTagAt(t, tg, entity.Ref{ID: "FEAT-1"}, "sync/base", 1)
	})

	t.Run("ReusesTheSweptVersion", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		meta, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-1"}, "base"))
		require.NoError(t, err)
		require.Equal(t, 2, meta.Version)
		require.Equal(t, store.VersionOpUpdate, meta.Op)
		metas, err := versionsOf(t, s).ListVersions(ctx(), entity.Ref{ID: "FEAT-1"})
		require.NoError(t, err)
		require.Len(t, metas, 2)
	})

	t.Run("CapturedUpdateIsAnUpdate", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "first")))
		h.sweepNow(t, s)
		require.NoError(t, s.UpdateEntity(ctx(), feature("FEAT-1", "second")))
		meta, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-1"}, "base"))
		require.NoError(t, err)
		require.Equal(t, 2, meta.Version)
		require.Equal(t, store.VersionOpUpdate, meta.Op)
		require.Equal(t, store.SweepPrincipalTool, meta.PrincipalTool, "an unattributed write falls back to the sweep")
	})

	t.Run("MissingEntityIsNotFound", func(t *testing.T) {
		tg := taggerOf(t, h.f(t))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-404"}, "base"))
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("NilProjectionIsRejected", func(t *testing.T) {
		p, ok := versionsOf(t, h.f(t)).(store.VersionTaggerProvider)
		require.True(t, ok)
		_, err := p.VersionTagger(nil)
		require.Error(t, err)
	})
}

// AC2, AC4 and R5.
func (h *tagHarness) runMoveTests(t *testing.T) {
	ref := entity.Ref{ID: "FEAT-1"}

	t.Run("TagMoveAndUntag", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)

		_, err := tg.TagVersion(ctx(), tagAt(t, ref, "base", 1))
		require.NoError(t, err)
		_, err = tg.TagVersion(ctx(), tagAt(t, ref, "approved", 1))
		require.NoError(t, err)
		requireTagAt(t, tg, ref, "base", 1)
		require.Equal(t, map[int][]string{1: {"approved", "base"}}, tagsByVersion(t, s, ref))

		meta, err := tg.TagVersion(ctx(), tagAt(t, ref, "base", 2))
		require.NoError(t, err)
		require.Equal(t, []string{"base"}, meta.Tags)
		requireTagAt(t, tg, ref, "base", 2)

		// The lookup serves the snapshot itself, the one GetVersion serves at
		// that ordinal, and the version service answers it without a
		// projection.
		lookup, ok := versionsOf(t, s).(store.VersionTagLookup)
		require.True(t, ok, "a tagging VersionService must be a store.VersionTagLookup")
		snap, err := lookup.VersionByTag(ctx(), ref, mustTagName(t, "base"))
		require.NoError(t, err)
		require.Equal(t, "second", snap.Content)
		require.Equal(t, 2, snap.Version)
		require.Equal(t, store.VersionOpUpdate, snap.Op)
		require.Equal(t, contentOf(t, s, ref, 2), snap.Content)
		require.Equal(t, map[int][]string{1: {"approved"}, 2: {"base"}}, tagsByVersion(t, s, ref))

		require.NoError(t, tg.UntagVersion(ctx(), untagReq(t, ref, "base")))
		requireNoTag(t, tg, ref, "base")
		require.ErrorIs(t, tg.UntagVersion(ctx(), untagReq(t, ref, "base")), store.ErrNotFound)
		require.NoError(t, tg.UntagVersion(ctx(), untagReq(t, ref, "approved")))
		require.Empty(t, tagsByVersion(t, s, ref))
	})

	t.Run("OrdinalOutOfRangeIsNotFound", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		_, err := tg.TagVersion(ctx(), tagAt(t, ref, "base", 3))
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("TagsArePerFace", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		draft := entity.Ref{ID: "FEAT-1", Face: "draft"}
		published := entity.Ref{ID: "FEAT-1", Face: "published"}
		require.NoError(t, s.CreateEntity(ctx(), faced("FEAT-1", "draft", "d")))
		require.NoError(t, s.CreateEntity(ctx(), faced("FEAT-1", "published", "p")))
		h.sweepNow(t, s)

		_, err := tg.TagCurrent(ctx(), tagReq(t, draft, "base"))
		require.NoError(t, err)
		requireTagAt(t, tg, draft, "base", 1)
		requireNoTag(t, tg, published, "base")

		_, err = tg.TagCurrent(ctx(), tagReq(t, published, "base"))
		require.NoError(t, err)
		require.NoError(t, tg.UntagVersion(ctx(), untagReq(t, draft, "base")))
		requireNoTag(t, tg, draft, "base")
		requireTagAt(t, tg, published, "base", 1)
	})

	t.Run("ConcurrentMovesLeaveOneTag", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		const writers = 8
		var wg sync.WaitGroup
		errs := make(chan error, writers)
		for i := range writers {
			wg.Go(func() {
				_, err := tg.TagVersion(ctx(), tagAt(t, ref, "base", 1+i%2))
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		n := 0
		for _, tags := range tagsByVersion(t, s, ref) {
			for _, name := range tags {
				if name == "base" {
					n++
				}
			}
		}
		require.Equal(t, 1, n, "a tag name names at most one version")
	})
}

// AC3 and R2.
func (h *tagHarness) runLifecycleTests(t *testing.T) {
	t.Run("RenameKeepsTheTag", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "body")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		renameAToBWithVersion(t, s, "body")
		requireTagAt(t, tg, entity.Ref{ID: "FEAT-B"}, "base", 1)

		// A later entity on the freed id does not inherit the tag.
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "unrelated")))
		h.sweepNow(t, s)
		requireNoTag(t, tg, entity.Ref{ID: "FEAT-A"}, "base")
	})

	t.Run("RecreationDoesNotInherit", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		ref := entity.Ref{ID: "FEAT-1"}
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "body")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, ref, "base"))
		require.NoError(t, err)
		deleteWithVersion(t, s, "FEAT-1")
		requireNoTag(t, tg, ref, "base")

		// Same bytes again: the new lifecycle gets its own version, and the
		// old one stays untaggable.
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "body")))
		h.sweepNow(t, s)
		requireNoTag(t, tg, ref, "base")
		require.Empty(t, tagsByVersion(t, s, ref), "a tag from an ended lifecycle is not listed")
		_, err = tg.TagVersion(ctx(), tagAt(t, ref, "base", 1))
		require.ErrorIs(t, err, store.ErrVersionNotTaggable, "a version from an ended lifecycle")

		meta, err := tg.TagCurrent(ctx(), tagReq(t, ref, "base"))
		require.NoError(t, err)
		require.Equal(t, store.VersionOpCreate, meta.Op)
		requireTagAt(t, tg, ref, "base", meta.Version)
	})

	t.Run("RenameOntoADeletedIDKeepsItsOwnTags", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-B", "old b")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-B"}, "old"))
		require.NoError(t, err)
		deleteWithVersion(t, s, "FEAT-B")

		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err = tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "kept"))
		require.NoError(t, err)
		renameAToBWithVersion(t, s, "a")

		got, err := tg.VersionByTag(ctx(), entity.Ref{ID: "FEAT-B"}, mustTagName(t, "kept"))
		require.NoError(t, err, "the renamed entity keeps its tag")
		require.Equal(t, "a", got.Content)
		require.Equal(t, "a", contentOf(t, s, entity.Ref{ID: "FEAT-B"}, got.Version))
		requireNoTag(t, tg, entity.Ref{ID: "FEAT-B"}, "old")
	})

	// The rename target's earlier occupant is deleted AFTER the renamed
	// entity was tagged: that delete is newer than the tagged row but belongs
	// to the old occupant, so it does not end the renamed chain.
	t.Run("RenameOntoAnIDDeletedAfterTheTag", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "kept"))
		require.NoError(t, err)

		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-B", "old b")))
		h.sweepNow(t, s)
		deleteWithVersion(t, s, "FEAT-B")

		renameAToBWithVersion(t, s, "a")
		got, err := tg.VersionByTag(ctx(), entity.Ref{ID: "FEAT-B"}, mustTagName(t, "kept"))
		require.NoError(t, err, "the renamed entity keeps its tag")
		require.Equal(t, "a", got.Content)
		require.Contains(t, tagsByVersion(t, s, entity.Ref{ID: "FEAT-B"})[got.Version], "kept")
	})

	t.Run("DeleteAfterRenameDropsTheTag", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		renameAToBWithVersion(t, s, "a")
		requireTagAt(t, tg, entity.Ref{ID: "FEAT-B"}, "base", 1)

		deleteWithVersion(t, s, "FEAT-B")
		requireNoTag(t, tg, entity.Ref{ID: "FEAT-B"}, "base")
		require.Empty(t, tagsByVersion(t, s, entity.Ref{ID: "FEAT-B"}))
		requireNoTag(t, tg, entity.Ref{ID: "FEAT-A"}, "base")
	})

	// A delete racing TagCurrent: the tag lands on the lineage before the
	// delete, or tagging fails. A capture after the delete row would revive
	// the deleted state as the newest version of its lineage.
	t.Run("ConcurrentDeleteAndTag", func(t *testing.T) {
		const rounds = 8
		for i := range rounds {
			s := h.f(t)
			tg := taggerOf(t, s)
			id := "FEAT-1"
			ref := entity.Ref{ID: id}
			// Unswept, so TagCurrent has to capture.
			require.NoError(t, s.CreateEntity(ctx(), feature(id, "body")))

			var (
				wg             sync.WaitGroup
				tagMeta        store.VersionMeta
				tagErr, delErr error
			)
			wg.Go(func() { tagMeta, tagErr = tg.TagCurrent(ctx(), tagReq(t, ref, "base")) })
			wg.Go(func() {
				if _, err := s.DeleteFamily(ctx(), id, true); err != nil {
					delErr = err
					return
				}
				delErr = versionsOf(t, s).WriteVersion(ctx(), store.VersionInput{
					EntityID: id, Op: store.VersionOpDelete, Type: "feature",
					SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
					PrincipalUser: "alice", PrincipalTool: "test",
				})
			})
			wg.Wait()
			require.NoError(t, delErr, "round %d", i)

			metas, err := versionsOf(t, s).ListVersions(ctx(), ref)
			require.NoError(t, err)
			deleteAt := 0
			for _, m := range metas {
				if m.Op == store.VersionOpDelete {
					deleteAt = m.Version
				}
			}
			require.NotZero(t, deleteAt, "round %d: no delete row", i)
			for _, m := range metas {
				if m.Op != store.VersionOpDelete {
					require.Less(t, m.Version, deleteAt,
						"round %d: a %s row was captured after the delete", i, m.Op)
				}
			}
			if tagErr == nil {
				require.Less(t, tagMeta.Version, deleteAt, "round %d: tagged after the delete", i)
			} else {
				require.ErrorIs(t, tagErr, store.ErrNotFound, "round %d", i)
			}

			// Whichever won, a recreated id inherits nothing.
			require.NoError(t, s.CreateEntity(ctx(), feature(id, "body")))
			h.sweepNow(t, s)
			requireNoTag(t, tg, ref, "base")
		}
	})
}

// contentOf reads the content of version ord of ref.
func contentOf(t *testing.T, s store.Store, ref entity.Ref, ord int) string {
	t.Helper()
	v, err := versionsOf(t, s).GetVersion(ctx(), ref, ord)
	require.NoError(t, err)
	return v.Content
}

// R1, R6 and R7.
func (h *tagHarness) runGuardTests(t *testing.T) {
	ref := entity.Ref{ID: "FEAT-1"}

	t.Run("ExpectMismatchIsAConflict", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "first")))
		old, err := s.GetEntity(ctx(), ref)
		require.NoError(t, err)
		stale := store.VersionOf(old)
		require.NoError(t, s.UpdateEntity(ctx(), feature("FEAT-1", "second")))

		req := tagReq(t, ref, "base")
		req.Expect = stale
		_, err = tg.TagCurrent(ctx(), req)
		require.ErrorIs(t, err, store.ErrConflict)
		var conflict *store.VersionConflictError
		require.True(t, errors.As(err, &conflict))
		requireNoTag(t, tg, ref, "base")

		cur, err := s.GetEntity(ctx(), ref)
		require.NoError(t, err)
		req.Expect = store.VersionOf(cur)
		meta, err := tg.TagCurrent(ctx(), req)
		require.NoError(t, err)
		require.Equal(t, "second", contentOf(t, s, ref, meta.Version))
	})

	t.Run("RefusedInsideATransaction", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		inTx := store.ContextInTx(ctx())
		_, err := tg.TagCurrent(inTx, tagReq(t, ref, "base"))
		require.ErrorIs(t, err, store.ErrTagInTx)
		_, err = tg.TagVersion(inTx, tagAt(t, ref, "base", 1))
		require.ErrorIs(t, err, store.ErrTagInTx)
		require.ErrorIs(t, tg.UntagVersion(inTx, untagReq(t, ref, "base")), store.ErrTagInTx)
	})

	t.Run("InvalidRequestsAreRefused", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		_, err := tg.TagCurrent(ctx(), store.TagRequest{Ref: ref, PrincipalTool: "connector"})
		require.ErrorIs(t, err, store.ErrInvalidVersionTag, "missing name")
		_, err = tg.TagCurrent(ctx(), store.TagRequest{Ref: ref, Name: mustTagName(t, "base")})
		require.ErrorIs(t, err, store.ErrInvalidVersionTag, "missing principal")
		_, err = tg.TagVersion(ctx(), tagReq(t, ref, "base"))
		require.ErrorIs(t, err, store.ErrInvalidVersionTag, "missing ordinal")
	})

	t.Run("DeleteRowIsNotTaggable", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "body")))
		h.sweepNow(t, s)
		deleteWithVersion(t, s, "FEAT-1")
		_, err := tg.TagVersion(ctx(), tagAt(t, ref, "base", 2))
		require.ErrorIs(t, err, store.ErrVersionNotTaggable)
	})

	t.Run("PurgedCurrentStateIsNotTaggable", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-1", "secret")))
		h.sweepNow(t, s)
		res, err := versionsOf(t, s).PurgeVersions(ctx(), store.VersionPurgeRequest{
			Ref: ref, Selector: store.PurgeSelector{All: true}, ForceLive: true, Reason: "test",
		})
		require.NoError(t, err)
		require.True(t, res.TombstoneWritten)

		// The tombstone must hold the live hash, or the sweep re-captures the
		// erased content and the tag below would find a version to name.
		h.sweepNow(t, s)
		metas, err := versionsOf(t, s).ListVersions(ctx(), ref)
		require.NoError(t, err)
		require.Len(t, metas, 1, "the sweep re-captured purged content")

		_, err = tg.TagCurrent(ctx(), tagReq(t, ref, "base"))
		require.ErrorIs(t, err, store.ErrVersionNotTaggable, "tagging must not re-capture purged content")
		_, err = tg.TagVersion(ctx(), tagAt(t, ref, "base", 1))
		require.ErrorIs(t, err, store.ErrVersionNotTaggable, "a purge tombstone")
	})
}

// AC6.
func (h *tagHarness) runPurgeTagTests(t *testing.T) {
	ref := entity.Ref{ID: "FEAT-1"}
	setup := func(t *testing.T) (store.Store, store.VersionTagger, int64) {
		t.Helper()
		s := h.f(t)
		tg := taggerOf(t, s)
		h.twoVersions(t, s)
		_, err := tg.TagVersion(ctx(), tagAt(t, ref, "base", 1))
		require.NoError(t, err)
		dry, err := versionsOf(t, s).PurgeVersions(ctx(), store.VersionPurgeRequest{
			Ref: ref, Selector: store.PurgeSelector{All: true}, DryRun: true, Reason: "test",
		})
		require.NoError(t, err)
		require.Len(t, dry.Targets, 2)
		return s, tg, dry.Targets[0].Vseq
	}
	purgeV1 := func(force bool) store.VersionPurgeRequest {
		// ForceLive: the live row still holds v2, and the live-row refusal
		// would otherwise mask the tag refusal under test.
		return store.VersionPurgeRequest{Ref: ref, Reason: "test", ForceLive: true, ForceTags: force}
	}

	t.Run("DryRunNamesTheTags", func(t *testing.T) {
		s, _, v1 := setup(t)
		req := purgeV1(false)
		req.Selector = store.PurgeSelector{Vseq: v1}
		req.DryRun = true
		res, err := versionsOf(t, s).PurgeVersions(ctx(), req)
		require.NoError(t, err)
		require.Equal(t, []store.PurgeTag{{Vseq: v1, Name: "base"}}, res.TaggedTargets)
		require.Zero(t, res.Purged)
	})

	t.Run("RefusesATaggedTarget", func(t *testing.T) {
		s, tg, v1 := setup(t)
		req := purgeV1(false)
		req.Selector = store.PurgeSelector{Vseq: v1}
		res, err := versionsOf(t, s).PurgeVersions(ctx(), req)
		require.NoError(t, err, "a refusal is a result, not an error")
		require.Zero(t, res.Purged)
		require.Len(t, res.TaggedTargets, 1)
		requireTagAt(t, tg, ref, "base", 1)
	})

	t.Run("ForceTagsDropsTheTag", func(t *testing.T) {
		s, tg, v1 := setup(t)
		req := purgeV1(true)
		req.Selector = store.PurgeSelector{Vseq: v1}
		res, err := versionsOf(t, s).PurgeVersions(ctx(), req)
		require.NoError(t, err)
		require.Equal(t, 1, res.Purged)
		requireNoTag(t, tg, ref, "base")
	})

	t.Run("UntaggedTargetsPurgeAsBefore", func(t *testing.T) {
		s, tg, _ := setup(t)
		metas, err := versionsOf(t, s).ListVersions(ctx(), ref)
		require.NoError(t, err)
		require.Len(t, metas, 2)
		dry, err := versionsOf(t, s).PurgeVersions(ctx(), store.VersionPurgeRequest{
			Ref: ref, Selector: store.PurgeSelector{All: true}, DryRun: true, Reason: "test",
		})
		require.NoError(t, err)
		req := purgeV1(false)
		req.Selector = store.PurgeSelector{Vseq: dry.Targets[1].Vseq}
		res, err := versionsOf(t, s).PurgeVersions(ctx(), req)
		require.NoError(t, err)
		require.Empty(t, res.TaggedTargets)
		require.Equal(t, 1, res.Purged, "v2 carries no tag")
		requireTagAt(t, tg, ref, "base", 1)
	})
}
