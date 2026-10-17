package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// cancellingTx cancels the request's context and fails, the way a client
// that disconnects while its write runs looks to the manager.
type cancellingTx struct {
	store.Store
	cancel context.CancelFunc
}

func (s cancellingTx) Tx(context.Context, func(store.Store) error) error {
	s.cancel()
	return errTxFailed
}

// The channels a failed write detached are restored even when the request
// was cancelled: the restore does not run on the request's context.
func TestAnchorGuard_UndoSurvivesCancelledRequest(t *testing.T) {
	t.Parallel()
	mem := memstore.New()
	ctx := context.Background()
	require.NoError(t, mem.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, mem.CreateEntity(ctx,
		&entity.Entity{ID: "REQ-1", Type: "requirement", Face: "published", Properties: map[string]any{}}))
	require.NoError(t, mem.CreateEntity(ctx, entity.New("auth", "component")))

	ops := []struct {
		name string
		run  func(context.Context, *entitymanager.Manager) error
		want string
	}{
		{"delete", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.DeleteEntity(ctx, "REQ-1", true)
			return err
		}, "REQ-1"},
		{"delete face", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.DeleteEntityFace(ctx, "REQ-1", "published", true)
			return err
		}, "REQ-1@published"},
		{"rename", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.RenameEntity(ctx, "auth", "gateway", entity.RenameOptions{})
			return err
		}, "rename:gateway"},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()
			reqCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			g := &fakeGuard{}
			mgr := newGuardedManager(t, cancellingTx{Store: mem, cancel: cancel}, g, audit.Nop{})
			require.ErrorIs(t, op.run(reqCtx, mgr), errTxFailed)
			assert.Equal(t, []string{op.want}, g.undoneRefs())
			assert.Equal(t, []error{nil}, g.undoCtxErrs, "the restore ran on a live context")
		})
	}
}

// inactiveGuard wraps a working guard but reports it inactive, the way a
// wrapped NopAnchorGuard does.
type inactiveGuard struct{ *fakeGuard }

func (inactiveGuard) Active() bool { return false }

// The guard steps run only for an active guard, and the pre-pass only when
// an entity the delete removes has channels. Activity is the guard's own
// answer, so a pointer or wrapped NopAnchorGuard is inactive too.
func TestAnchorGuard_SkippedWithoutChannelsOrWhenInactive(t *testing.T) {
	t.Parallel()
	assert.False(t, (&entitymanager.NopAnchorGuard{}).Active())

	t.Run("inactive", func(t *testing.T) {
		t.Parallel()
		g := &fakeGuard{blockWhole: true}
		f := newOwnedDeleteFixtureWith(t, concBackends[0], inactiveGuard{g})
		_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
		require.NoError(t, err)
		assert.Empty(t, g.scopes)
		assert.Empty(t, g.deletes)
	})

	t.Run("no channels", func(t *testing.T) {
		t.Parallel()
		g := &fakeGuard{blockWhole: true, noChannels: true}
		f := newOwnedDeleteFixtureWith(t, concBackends[0], g)
		_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
		require.NoError(t, err)
		require.Len(t, g.scopes, 1)
		assert.ElementsMatch(t, []string{f.owner, f.c1, f.c2}, g.scopes[0],
			"the scope covers the owner and what it owns")
		assert.Empty(t, g.deletes)
	})
}

// A caller who may not delete an owned entity is refused for that, never
// told about a blocking channel, on the hard and the soft path alike.
func TestAnchorGuard_OwnedDenialComesBeforeTheBlock(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			g := &fakeGuard{blockWhole: true}
			f := newOwnedDeleteFixtureWith(t, b, g, "c2")
			var blocked *entitymanager.AnchorBlockedError

			_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			require.ErrorIs(t, err, acl.ErrForbidden)
			assert.NotErrorAs(t, err, &blocked)

			if entitymanager.SupportsSoftDelete(f.mgr) {
				_, err = entitymanager.SoftDeleteEntity(aliceCtx(), f.mgr, f.owner)
				require.ErrorIs(t, err, acl.ErrForbidden)
				assert.NotErrorAs(t, err, &blocked)
			}
			assert.Empty(t, g.deletes)
			assert.Empty(t, g.checks)
		})
	}
}

// A blocking channel on an owned entity refuses the owner's delete. The
// refusal names neither the owned entity nor anything about it but the
// channel type, and the owner's detached channels are put back. A guard
// failure on the owned entity is reported without its id too.
func TestAnchorGuard_OwnedChildRefusals(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			g := &fakeGuard{}
			f := newOwnedDeleteFixtureWith(t, b, g)
			g.blockIDs = map[string]bool{f.c1: true}

			_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			var blocked *entitymanager.AnchorBlockedError
			require.ErrorAs(t, err, &blocked)
			assert.NotContains(t, err.Error(), f.c1)
			assert.Equal(t, []string{f.owner}, g.undoneRefs())
			for _, id := range []string{f.owner, f.c1, f.c2} {
				assert.True(t, f.exists(t, id), "%s survives", id)
			}

			if entitymanager.SupportsSoftDelete(f.mgr) {
				_, err = entitymanager.SoftDeleteEntity(aliceCtx(), f.mgr, f.owner)
				require.ErrorAs(t, err, &blocked)
				assert.NotContains(t, err.Error(), f.c1)
			}

			g.blockIDs = nil
			g.errIDs = map[string]bool{f.c2: true}
			_, err = f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), f.owner)
			assert.NotContains(t, err.Error(), f.c2)
			assert.True(t, f.exists(t, f.c2))
		})
	}
}

// An owning edge added after the guard ran would take an entity along whose
// channels nobody checked. The transaction refuses it as a retryable
// conflict and the detached channels are put back.
func TestAnchorGuard_OwnedRaceIsConflict(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			var (
				once sync.Once
				st   store.Store
				own  string
			)
			g := &fakeGuard{onDelete: func(entitymanager.AnchorDelete) {
				once.Do(func() {
					ctx := context.Background()
					require.NoError(t, st.CreateEntity(ctx, entity.New("NOTE-99", "note")))
					_, err := st.CreateRelation(ctx, entity.RelationKey{From: own, Type: "part", To: "NOTE-99"}, nil)
					require.NoError(t, err)
				})
			}}
			f := newOwnedDeleteFixtureWith(t, b, g)
			st, own = f.st, f.owner

			_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			require.ErrorIs(t, err, entitymanager.ErrConcurrentChange)
			require.NotErrorIs(t, err, store.ErrConflict)
			assert.Len(t, g.undoneRefs(), 3, "every detach is put back")
			for _, id := range []string{f.owner, f.c1, f.c2, "NOTE-99"} {
				assert.True(t, f.exists(t, id), "%s survives", id)
			}
		})
	}
}

// A face delete whose last-face answer changes after the guard acted is a
// retryable conflict, and the face's channels are put back.
func TestAnchorGuard_FacesChangedIsConflict(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			var (
				once sync.Once
				st   store.Store
			)
			g := &fakeGuard{onDelete: func(entitymanager.AnchorDelete) {
				once.Do(func() {
					_, err := st.DeleteFace(context.Background(), entity.Ref{ID: "POL-1", Face: "published"})
					require.NoError(t, err)
				})
			}}
			f := newFaceEdgeFixtureWith(t, b, g)
			st = f.st

			_, err := f.mgr.DeleteEntityFace(asUser("admin"), "POL-1", "draft", true)
			require.ErrorIs(t, err, entitymanager.ErrConcurrentChange)
			require.NotErrorIs(t, err, store.ErrConflict)
			assert.Equal(t, []string{"POL-1@draft"}, g.undoneRefs())
			_, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"})
			require.NoError(t, gErr)
		})
	}
}

// The guard's conflict checks run whenever the delete's entities have
// channels, not only when it detached some: a block-only scope was judged
// on the same answers. A sibling face that goes, or an owned entity that
// appears, after the scope was taken is a retryable conflict even though
// nothing was detached.
func TestAnchorGuard_ConflictWithoutDetach(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name+"/face", func(t *testing.T) {
			t.Parallel()
			var (
				once sync.Once
				st   store.Store
			)
			g := &fakeGuard{noDetach: true, onDelete: func(entitymanager.AnchorDelete) {
				once.Do(func() {
					_, err := st.DeleteFace(context.Background(), entity.Ref{ID: "POL-1", Face: "published"})
					require.NoError(t, err)
				})
			}}
			f := newFaceEdgeFixtureWith(t, b, g)
			st = f.st

			_, err := f.mgr.DeleteEntityFace(asUser("admin"), "POL-1", "draft", true)
			require.ErrorIs(t, err, entitymanager.ErrConcurrentChange)
			assert.Empty(t, g.undoneRefs(), "nothing was detached")
			_, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"})
			require.NoError(t, gErr)
		})
		t.Run(b.name+"/owned", func(t *testing.T) {
			t.Parallel()
			var (
				once sync.Once
				st   store.Store
				own  string
			)
			g := &fakeGuard{noDetach: true, onDelete: func(entitymanager.AnchorDelete) {
				once.Do(func() {
					ctx := context.Background()
					require.NoError(t, st.CreateEntity(ctx, entity.New("NOTE-99", "note")))
					_, err := st.CreateRelation(ctx, entity.RelationKey{From: own, Type: "part", To: "NOTE-99"}, nil)
					require.NoError(t, err)
				})
			}}
			f := newOwnedDeleteFixtureWith(t, b, g)
			st, own = f.st, f.owner

			_, err := f.mgr.DeleteEntity(aliceCtx(), f.owner, true)
			require.ErrorIs(t, err, entitymanager.ErrConcurrentChange)
			assert.Empty(t, g.undoneRefs(), "nothing was detached")
			for _, id := range []string{f.owner, f.c1, f.c2, "NOTE-99"} {
				assert.True(t, f.exists(t, id), "%s survives", id)
			}
		})
	}
}

// failingPurgeStore is a soft-delete store whose purge fails.
type failingPurgeStore struct{ *memstore.MemStore }

func (s failingPurgeStore) SoftDelete() store.SoftDeleter {
	return failingPurge{s.MemStore.SoftDelete()}
}

type failingPurge struct{ store.SoftDeleter }

var errPurge = errors.New("purge failed")

func (failingPurge) PurgeMarked(context.Context, string) (*store.DeleteResult, error) {
	return nil, errPurge
}

// A purge that fails, or finds the entity restored meanwhile, puts the
// channels it detached back: the entity they belong to is still there.
func TestAnchorGuard_PurgeUndo(t *testing.T) {
	t.Parallel()
	cutoff := func() time.Time { return time.Now().Add(time.Second) }

	t.Run("purge fails", func(t *testing.T) {
		t.Parallel()
		mem := memstore.New()
		seedAddressed(t, mem)
		g := &fakeGuard{}
		mgr := newGuardedManager(t, failingPurgeStore{mem}, g, audit.Nop{})
		_, err := entitymanager.SoftDeleteEntity(aliceCtx(), mgr, "REQ-1")
		require.NoError(t, err)

		n, err := entitymanager.PurgeSoftDeleted(context.Background(), mgr, cutoff())
		require.ErrorIs(t, err, errPurge)
		assert.Zero(t, n)
		assert.Equal(t, []string{"REQ-1"}, g.undoneRefs())
	})

	t.Run("restored meanwhile", func(t *testing.T) {
		t.Parallel()
		mem := memstore.New()
		seedAddressed(t, mem)
		g := &fakeGuard{onDelete: func(d entitymanager.AnchorDelete) {
			if d.Purge {
				_, err := mem.SoftDelete().Unmark(context.Background(), d.ID)
				require.NoError(t, err)
			}
		}}
		mgr := newGuardedManager(t, mem, g, audit.Nop{})
		_, err := entitymanager.SoftDeleteEntity(aliceCtx(), mgr, "REQ-1")
		require.NoError(t, err)

		n, err := entitymanager.PurgeSoftDeleted(context.Background(), mgr, cutoff())
		require.NoError(t, err)
		assert.Zero(t, n)
		assert.Equal(t, []string{"REQ-1"}, g.undoneRefs())
		_, gErr := mem.GetEntity(context.Background(), entity.Ref{ID: "REQ-1"})
		require.NoError(t, gErr)
	})

	t.Run("purged", func(t *testing.T) {
		t.Parallel()
		mem := memstore.New()
		seedAddressed(t, mem)
		g := &fakeGuard{}
		mgr := newGuardedManager(t, mem, g, audit.Nop{})
		_, err := entitymanager.SoftDeleteEntity(aliceCtx(), mgr, "REQ-1")
		require.NoError(t, err)
		n, err := entitymanager.PurgeSoftDeleted(context.Background(), mgr, cutoff())
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Empty(t, g.undoneRefs(), "a purge that happened keeps the detach")
	})
}

// A rename whose transaction fails puts back the channels the guard
// cleared from the new id.
func TestAnchorGuard_RenameTxFailureRestores(t *testing.T) {
	t.Parallel()
	mem := memstore.New()
	require.NoError(t, mem.CreateEntity(context.Background(), entity.New("auth", "component")))
	g := &fakeGuard{}
	mgr := newGuardedManager(t, failingTx{mem}, g, audit.Nop{})
	_, err := mgr.RenameEntity(context.Background(), "auth", "gateway", entity.RenameOptions{})
	require.ErrorIs(t, err, errTxFailed)
	assert.Equal(t, []string{"gateway"}, g.detached)
	assert.Equal(t, []string{"rename:gateway"}, g.undoneRefs())
	assert.NotContains(t, strings.Join(g.undoneRefs(), ","), "auth")
}

// An entity that takes the new id after a failed rename put its channels
// back gets them detached again: they belong to the earlier entity, not to
// the new one.
func TestAnchorGuard_RenameUndoRechecksNewID(t *testing.T) {
	t.Parallel()
	mem := memstore.New()
	require.NoError(t, mem.CreateEntity(context.Background(), entity.New("auth", "component")))
	g := &fakeGuard{}
	g.onRestore = func() {
		require.NoError(t, mem.CreateEntity(context.Background(), entity.New("gateway", "component")))
	}
	mgr := newGuardedManager(t, failingTx{mem}, g, audit.Nop{})
	_, err := mgr.RenameEntity(context.Background(), "auth", "gateway", entity.RenameOptions{})
	require.ErrorIs(t, err, errTxFailed)
	assert.Equal(t, []string{"rename:gateway"}, g.undoneRefs())
	assert.Equal(t, []string{"gateway", "gateway"}, g.detached, "the restored channels are detached again")
}
