package storetest

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// requireTaggedConsistent checks rows, the TaggedEntities answer for name,
// against the Go lineage reads: a row's Version is a version
// ListVersions(Ref) lists with the tag, or 0 only for a deleted row whose
// listed history lost the tagged version; a live row is the live face of its
// type, and VersionByTag answers the same ordinal.
func requireTaggedConsistent(t *testing.T, s store.Store, name string, rows []store.TaggedEntity) {
	t.Helper()
	vs := versionsOf(t, s)
	tg := taggerOf(t, s)
	tag := mustTagName(t, name)
	for _, row := range rows {
		metas, err := vs.ListVersions(ctx(), row.Ref)
		require.NoError(t, err, "ListVersions(%v)", row.Ref)
		if row.Version == 0 {
			require.True(t, row.Deleted, "%v: only a deleted row may lack an ordinal", row.Ref)
		} else {
			require.LessOrEqual(t, row.Version, len(metas), "%v: ordinal past the listed history", row.Ref)
			meta := metas[row.Version-1]
			require.Equal(t, row.Version, meta.Version, "%v: listed ordinal", row.Ref)
			if !row.Deleted {
				// ListVersions shows the current lifecycle's tags only, so a
				// deleted row's tag cannot be checked against it.
				require.Contains(t, meta.Tags, name, "%v: listed version %d does not carry the tag", row.Ref, row.Version)
			}
		}
		if row.Deleted {
			continue
		}
		live, err := s.GetEntity(ctx(), row.Ref)
		require.NoError(t, err, "%v: a live row must have a live face", row.Ref)
		require.Equal(t, live.Type, row.Type, "%v: live type", row.Ref)
		snap, err := tg.VersionByTag(ctx(), row.Ref, tag)
		require.NoError(t, err, "%v: VersionByTag", row.Ref)
		require.Equal(t, snap.Version, row.Version, "%v: VersionByTag ordinal", row.Ref)
	}
}

// renameWithVersion renames from to to and writes the rename version.
func renameWithVersion(t *testing.T, s store.Store, from, to, body string) {
	t.Helper()
	_, err := s.RenameFamily(ctx(), from, to)
	require.NoError(t, err)
	writeVersion(t, versionsOf(t, s), store.VersionInput{
		EntityID: to, Op: store.VersionOpRename, PrevID: from, Type: "feature", Content: body,
		PrincipalUser: "alice", PrincipalTool: "test",
	})
}

// runTaggedLineageTests covers the rename shapes the hand-picked cases do
// not: a chain, a rename back, and random histories. [taggedEntities] checks
// every row against ListVersions and VersionByTag.
func (h *tagHarness) runTaggedLineageTests(t *testing.T) {
	t.Run("RenameChain", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-A"}, "base"))
		require.NoError(t, err)
		renameWithVersion(t, s, "FEAT-A", "FEAT-B", "a")
		renameWithVersion(t, s, "FEAT-B", "FEAT-C", "a")
		got := taggedEntities(t, s, "base", "")
		require.Equal(t, []store.TaggedEntity{tagged(entity.Ref{ID: "FEAT-C"}, "feature", 1, false)}, got)
	})

	t.Run("RenameBack", func(t *testing.T) {
		s := h.f(t)
		tg := taggerOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), feature("FEAT-A", "a")))
		h.sweepNow(t, s)
		renameWithVersion(t, s, "FEAT-A", "FEAT-B", "a")
		_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: "FEAT-B"}, "base"))
		require.NoError(t, err)
		renameWithVersion(t, s, "FEAT-B", "FEAT-A", "a")
		got := taggedEntities(t, s, "base", "")
		require.Len(t, got, 1)
		require.Equal(t, entity.Ref{ID: "FEAT-A"}, got[0].Ref)
		require.False(t, got[0].Deleted)
	})

	t.Run("Randomized", func(t *testing.T) {
		for seed := range uint64(12) {
			t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) { h.runRandomTagHistory(t, seed) })
		}
	})
}

// lifecycle is one entity from its create to its delete in the model of
// runRandomTagHistory.
type lifecycle struct {
	id      string
	deleted bool
}

// runRandomTagHistory applies a random history of creates, updates, renames,
// deletes and tags over three ids, then checks every tag against the model
// (where the tagged lifecycle lives now, and whether it ended) and against
// the Go lineage reads.
func (h *tagHarness) runRandomTagHistory(t *testing.T, seed uint64) {
	s := h.f(t)
	tg := taggerOf(t, s)
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	ids := []string{"FEAT-1", "FEAT-2", "FEAT-3"}
	alive := map[string]*lifecycle{}
	var tags []*lifecycle // tags[k] is the lifecycle tag t<k> was put on
	body := 0
	next := func() string { body++; return fmt.Sprint("b", body) }

	for range 18 {
		id := ids[rng.IntN(len(ids))]
		lc := alive[id]
		switch op := rng.IntN(5); {
		case lc == nil && op < 4:
			require.NoError(t, s.CreateEntity(ctx(), feature(id, next())))
			h.sweepNow(t, s)
			alive[id] = &lifecycle{id: id}
		case lc == nil:
			continue
		case op == 0:
			require.NoError(t, s.UpdateEntity(ctx(), feature(id, next())))
			h.sweepNow(t, s)
		case op == 1:
			to := ids[rng.IntN(len(ids))]
			if alive[to] != nil {
				continue
			}
			renameWithVersion(t, s, id, to, fmt.Sprint("b", body))
			delete(alive, id)
			lc.id = to
			alive[to] = lc
		case op == 2:
			deleteWithVersion(t, s, id)
			delete(alive, id)
			lc.deleted = true
		default:
			name := fmt.Sprint("t", len(tags))
			_, err := tg.TagCurrent(ctx(), tagReq(t, entity.Ref{ID: id}, name))
			if errors.Is(err, store.ErrNotFound) {
				continue // no captured version yet
			}
			require.NoError(t, err)
			tags = append(tags, lc)
		}
	}

	for k, lc := range tags {
		name := fmt.Sprint("t", k)
		got := taggedEntities(t, s, name, "")
		require.Len(t, got, 1, "tag %s", name)
		require.Equal(t, lc.id, got[0].Ref.ID, "tag %s: where the lifecycle lives", name)
		require.Equal(t, lc.deleted, got[0].Deleted, "tag %s: whether the lifecycle ended", name)
	}
}
