package storetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunSweepOriginTests pins the provenance a swept create/update version
// carries. The sweep captures from the live row, so a backend must keep the
// write's [store.Origin] on the row for the version to name it (BUG-YC5Z07).
//
// sweepNow must run one tick that treats every row as settled.
func RunSweepOriginTests(t *testing.T, f Factory, sweepNow func(t *testing.T, s store.Store)) {
	copied := store.Origin{
		Kind:       store.OriginCopy,
		Source:     "POL-1",
		SourceFace: "draft",
		SourceType: "policy",
		Definition: "publish",
	}
	editor := store.Attribution{User: "edith", Tool: "data-entry"}
	policy := func(id, body string) *entity.Entity {
		e := entity.New(id, "policy")
		e.Content = body
		return e
	}
	onlyVersion := func(t *testing.T, v store.VersionService, id string) store.VersionMeta {
		t.Helper()
		metas, err := v.ListVersions(ctx(), id)
		require.NoError(t, err)
		require.Len(t, metas, 1, "versions of %s", id)
		return metas[0]
	}

	// One sweep over a copied and a hand-written entity, with the same editor,
	// so the origin is the only thing that can tell them apart. A backend that
	// stamped every version alike would fail one of the two.
	t.Run("CopyIsDistinguishableFromAHandEdit", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		handCtx := store.WithAttribution(context.Background(), editor)
		require.NoError(t, s.CreateEntity(store.WithOrigin(handCtx, copied), policy("POL-2", "copied body")))
		require.NoError(t, s.CreateEntity(handCtx, policy("POL-3", "typed body")))
		sweepNow(t, s)

		cv := onlyVersion(t, v, "POL-2")
		require.Equal(t, copied, cv.Origin)
		require.Equal(t, store.VersionOpCreate, cv.Op, "a copy is still a create of the target row")
		require.Equal(t, "POL-1@draft", cv.Origin.SourceLabel())

		hand := onlyVersion(t, v, "POL-3")
		require.True(t, hand.Origin.IsZero(), "a hand edit carries no origin")
		require.Equal(t, editor.User, hand.PrincipalUser)

		snap, err := v.GetVersion(ctx(), "POL-2", 1)
		require.NoError(t, err)
		require.Equal(t, copied, snap.Origin, "the snapshot read must agree with the timeline read")
	})

	// Publishing over an existing row is the common copy: an update, not a
	// create, still carrying the copy's origin.
	t.Run("CopyOntoAnExistingRowIsAnUpdate", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		require.NoError(t, s.CreateEntity(ctx(), policy("POL-5", "typed body")))
		sweepNow(t, s)
		require.NoError(t, s.UpdateEntity(store.WithOrigin(ctx(), copied), policy("POL-5", "copied body")))
		sweepNow(t, s)

		metas, err := v.ListVersions(ctx(), "POL-5")
		require.NoError(t, err)
		require.Len(t, metas, 2)
		require.True(t, metas[0].Origin.IsZero(), "v1 was typed by hand")
		require.Equal(t, store.VersionOpUpdate, metas[1].Op)
		require.Equal(t, copied, metas[1].Origin, "v2 was the copy")
	})

	// The origin describes the most recent write, so a hand edit of a copied
	// row must not inherit the copy marker.
	t.Run("HandEditClearsTheOrigin", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		require.NoError(t, s.CreateEntity(store.WithOrigin(ctx(), copied), policy("POL-4", "copied body")))
		sweepNow(t, s)
		require.NoError(t, s.UpdateEntity(ctx(), policy("POL-4", "hand-edited body")))
		sweepNow(t, s)

		metas, err := v.ListVersions(ctx(), "POL-4")
		require.NoError(t, err)
		require.Len(t, metas, 2)
		require.Equal(t, copied, metas[0].Origin, "v1 was the copy")
		require.True(t, metas[1].Origin.IsZero(), "v2 was typed by hand")
	})
}
