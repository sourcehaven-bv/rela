package entitymanager_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// racingTx runs race once, just before the first transaction opens: a write
// that lands after the caller read the row and before the delete. It embeds
// the memstore itself, so its optional capabilities (soft delete) stay.
type racingTx struct {
	*memstore.MemStore
	once *sync.Once
	race func()
}

func (s racingTx) Tx(ctx context.Context, fn func(store.Store) error) error {
	s.once.Do(s.race)
	return s.MemStore.Tx(ctx, fn)
}

// A conditional delete compares the version inside its own transaction: a
// write between the caller's read and the delete makes it fail with a
// version conflict and deletes nothing, and the version read just before
// still deletes when nothing lands in between (TKT-CVT9W1K).
func TestIfMatchDeletes_WriteBetweenReadAndDelete(t *testing.T) {
	t.Parallel()
	bump := func(t *testing.T, st store.Store, ref entity.Ref) func() {
		t.Helper()
		return func() {
			e, err := st.GetEntity(context.Background(), ref)
			require.NoError(t, err)
			e.Properties = map[string]any{"title": "changed"}
			require.NoError(t, st.UpdateEntity(context.Background(), e))
		}
	}
	entityOps := []struct {
		name string
		ref  entity.Ref
		run  func(entitymanager.IfMatchDeletes, entity.Ref, store.EntityVersion) error
	}{
		{"family", entity.Ref{ID: "REQ-1"}, func(d entitymanager.IfMatchDeletes, r entity.Ref, v store.EntityVersion) error {
			_, err := d.DeleteEntity(context.Background(), r, true, v)
			return err
		}},
		{"face", entity.Ref{ID: "REQ-1", Face: "published"},
			func(d entitymanager.IfMatchDeletes, r entity.Ref, v store.EntityVersion) error {
				_, err := d.DeleteEntity(context.Background(), r, true, v)
				return err
			}},
		{"soft", entity.Ref{ID: "REQ-1"}, func(d entitymanager.IfMatchDeletes, r entity.Ref, v store.EntityVersion) error {
			_, err := d.SoftDeleteEntity(context.Background(), r, v)
			return err
		}},
	}
	for _, op := range entityOps {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()
			for _, raced := range []bool{true, false} {
				mem := memstore.New()
				require.NoError(t, mem.CreateEntity(context.Background(), entity.New("REQ-1", "requirement")))
				require.NoError(t, mem.CreateEntity(context.Background(),
					&entity.Entity{ID: "REQ-1", Type: "requirement", Face: "published", Properties: map[string]any{}}))
				read, err := mem.GetEntity(context.Background(), op.ref)
				require.NoError(t, err)
				race := func() {}
				if raced {
					race = bump(t, mem, op.ref)
				}
				st := racingTx{MemStore: mem, once: &sync.Once{}, race: race}
				d := entitymanager.IfMatchDeletes{M: newGuardedManager(t, st, entitymanager.NopAnchorGuard{}, audit.Nop{})}
				err = op.run(d, op.ref, store.VersionOf(read))
				_, getErr := mem.GetEntity(context.Background(), op.ref)
				if raced {
					var conflict *store.VersionConflictError
					require.ErrorAs(t, err, &conflict)
					require.NoError(t, getErr, "a refused delete removed the row")
				} else {
					require.NoError(t, err)
					require.ErrorIs(t, getErr, store.ErrNotFound, "the delete left the row")
				}
			}
		})
	}
	t.Run("relation", func(t *testing.T) {
		t.Parallel()
		for _, raced := range []bool{true, false} {
			mem := memstore.New()
			seedAddressed(t, mem)
			key := entity.RelationKey{From: "DEC-1", Type: "addresses", To: "REQ-1"}
			read, err := mem.GetRelation(context.Background(), key)
			require.NoError(t, err)
			race := func() {}
			if raced {
				race = func() {
					_, uErr := mem.UpdateRelation(context.Background(), key, store.RelationData{Properties: map[string]any{"note": "changed"}})
					require.NoError(t, uErr)
				}
			}
			st := racingTx{MemStore: mem, once: &sync.Once{}, race: race}
			d := entitymanager.IfMatchDeletes{M: newGuardedManager(t, st, entitymanager.NopAnchorGuard{}, audit.Nop{})}
			err = d.DeleteRelation(context.Background(), key, store.RelationVersionOf(read))
			_, getErr := mem.GetRelation(context.Background(), key)
			if raced {
				var conflict *store.VersionConflictError
				require.ErrorAs(t, err, &conflict)
				require.NoError(t, getErr, "a refused delete removed the relation")
			} else {
				require.NoError(t, err)
				require.ErrorIs(t, getErr, store.ErrNotFound, "the delete left the relation")
			}
		}
	})
}
