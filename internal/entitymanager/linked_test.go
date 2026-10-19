package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// rollsBack reports whether backend b's Tx undoes the writes of a failed
// callback.
func rollsBack(b concBackend) bool {
	return b.name == "sqlite" || strings.HasPrefix(b.name, "postgres")
}

// TestCreateLinked_CommitsTogether pins that CreateLinked writes the
// entity and its edge, and runs InTx on a view that sees both.
func TestCreateLinked_CommitsTogether(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr := openConcBackend(t, backend)
			target := mustCreateNote(t, mgr, "target")
			e := entity.New("", "note")
			e.SetString("title", "new")
			var seen string
			res, err := entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
				Entity: e, RelateType: "links", RelateTo: target.ID,
				InTx: func(ctx context.Context, view store.Store, created *entity.Entity) error {
					got, err := view.GetEntity(ctx, created.Ref())
					if err != nil {
						return err
					}
					_, err = view.GetRelation(ctx, entity.RelationKey{From: created.ID, Type: "links", To: target.ID})
					seen = got.ID
					return err
				},
			})
			require.NoError(t, err)
			require.NoError(t, res.FollowUpErr)
			require.NoError(t, res.RelateErr)
			id := res.Create.Entity.ID
			require.Equal(t, id, seen)
			require.NotNil(t, res.Relation)
			_, err = st.GetRelation(context.Background(), entity.RelationKey{From: id, Type: "links", To: target.ID})
			require.NoError(t, err)
		})
	}
}

// TestCreateLinked_InTxErrorRollsBack pins the atomicity: when InTx fails,
// a store with a real transaction keeps neither the entity nor the edge.
// The filesystem and memory stores keep what they wrote.
func TestCreateLinked_InTxErrorRollsBack(t *testing.T) {
	boom := errors.New("mark failed")
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr := openConcBackend(t, backend)
			target := mustCreateNote(t, mgr, "target")
			e := entity.New("", "note")
			e.SetString("title", "new")
			var id string
			_, err := entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
				Entity: e, RelateType: "links", RelateTo: target.ID,
				InTx: func(_ context.Context, _ store.Store, created *entity.Entity) error {
					id = created.ID
					return boom
				},
			})
			require.ErrorIs(t, err, boom)
			_, getErr := st.GetEntity(context.Background(), entity.Ref{ID: id})
			_, relErr := st.GetRelation(context.Background(), entity.RelationKey{From: id, Type: "links", To: target.ID})
			if rollsBack(backend) {
				require.ErrorIs(t, getErr, store.ErrNotFound)
				require.ErrorIs(t, relErr, store.ErrNotFound)
			} else {
				require.NoError(t, getErr)
				require.NoError(t, relErr)
			}
		})
	}
}

// TestCreateLinked_RelationCheckedBeforeWrite pins that an edge the
// metamodel refuses fails before the entity is written, on every store.
func TestCreateLinked_RelationCheckedBeforeWrite(t *testing.T) {
	mgr, cs := newManager(t, nil)
	req := createReq(t, mgr, "r")
	before := cs.creates.Load()
	e := entity.New("", "decision")
	e.SetString("title", "d")
	_, err := entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
		Entity: e, RelateType: "covers", RelateTo: req.ID,
	})
	var invalid *entitymanager.InvalidRelationError
	require.ErrorAs(t, err, &invalid)
	_, err = entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
		Entity: e, RelateType: "addresses", RelateTo: "REQ-404",
	})
	require.ErrorIs(t, err, entitymanager.ErrEntityNotFound)
	require.Equal(t, before, cs.creates.Load(), "no entity is written")
}

// failingRelStore refuses every relation create, also on its Tx views.
type failingRelStore struct {
	store.Store
	err error
}

func (s *failingRelStore) CreateRelation(
	context.Context, entity.RelationKey, *store.RelationData,
) (*entity.Relation, error) {
	return nil, s.err
}

func (s *failingRelStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(view store.Store) error { return fn(&failingRelStore{Store: view, err: s.err}) })
}

// TestCreateLinked_RelateBestEffort pins that RelateBestEffort keeps the
// entity and reports the edge failure, and that without it the failure is
// the error.
func TestCreateLinked_RelateBestEffort(t *testing.T) {
	boom := errors.New("edge write failed")
	for _, bestEffort := range []bool{true, false} {
		mgr, cs := newManager(t, nil)
		req := createReq(t, mgr, "r")
		failing := entitymanager.WithStore(mgr, &failingRelStore{Store: cs, err: boom})
		e := entity.New("", "decision")
		e.SetString("title", "d")
		res, err := entitymanager.CreateLinked(context.Background(), failing, entitymanager.LinkedCreate{
			Entity: e, RelateType: "addresses", RelateTo: req.ID, RelateBestEffort: bestEffort,
		})
		if bestEffort {
			require.NoError(t, err)
			require.ErrorIs(t, res.RelateErr, boom)
			require.Nil(t, res.Relation)
			require.NotEmpty(t, res.Create.Entity.ID)
		} else {
			require.ErrorIs(t, err, boom)
		}
	}
}
