package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunEntityTests runs entity CRUD conformance tests.
func RunEntityTests(t *testing.T, f Factory) {
	t.Run("CreateAndGet", func(t *testing.T) {
		s := f(t)
		e := entity.New("FEAT-001", "feature")
		e.SetString("title", "Login")

		err := s.CreateEntity(ctx(), e)
		require.NoError(t, err)

		got, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-001"})
		require.NoError(t, err)
		assert.Equal(t, e.ID, got.ID)
		assert.Equal(t, e.Type, got.Type)
		assert.Equal(t, "Login", got.GetString("title"))
	})

	t.Run("GetNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.GetEntity(ctx(), entity.Ref{ID: "NOPE"})
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	// Entity.Redacted is a per-reader ACL artifact, not content: it must
	// never survive a write. Backend-agnostic because memstore may hold
	// structs directly rather than serializing through markdown, so a
	// markdown-only assertion would not cover it (RR-KBWJPV).
	//
	// A redacted entity should never reach a write path at all — write-prep
	// reads go through entitymanager.PatchEntity, which merges against the
	// raw stored entity — but the whole design rests on that separation, so
	// it is worth one cheap assertion that a slip does not persist someone's
	// per-principal view.
	t.Run("RedactedNotPersisted", func(t *testing.T) {
		s := f(t)
		e := entity.New("FEAT-002", "feature")
		e.SetString("title", "Login")
		e.Redacted = []string{"salary"}

		require.NoError(t, s.CreateEntity(ctx(), e))

		got, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-002"})
		require.NoError(t, err)
		assert.Empty(t, got.Redacted, "Redacted is a read-out artifact and must not round-trip")
	})

	t.Run("CreateConflict", func(t *testing.T) {
		s := f(t)
		e := entity.New("FEAT-001", "feature")
		require.NoError(t, s.CreateEntity(ctx(), e))

		err := s.CreateEntity(ctx(), entity.New("FEAT-001", "feature"))
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("GetReturnsClone", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "Original")
		require.NoError(t, s.CreateEntity(ctx(), e))

		got, _ := s.GetEntity(ctx(), entity.Ref{ID: "T-1"})
		got.SetString("title", "Mutated")

		got2, _ := s.GetEntity(ctx(), entity.Ref{ID: "T-1"})
		assert.Equal(t, "Original", got2.GetString("title"))
	})

	t.Run("CreateStoresClone", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "Before")
		require.NoError(t, s.CreateEntity(ctx(), e))

		e.SetString("title", "After")

		got, _ := s.GetEntity(ctx(), entity.Ref{ID: "T-1"})
		assert.Equal(t, "Before", got.GetString("title"))
	})

	t.Run("Update", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "v1")
		require.NoError(t, s.CreateEntity(ctx(), e))

		updated := entity.New("T-1", "ticket")
		updated.SetString("title", "v2")
		err := s.UpdateEntity(ctx(), updated)
		require.NoError(t, err)

		got, _ := s.GetEntity(ctx(), entity.Ref{ID: "T-1"})
		assert.Equal(t, "v2", got.GetString("title"))
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		s := f(t)
		err := s.UpdateEntity(ctx(), entity.New("NOPE", "ticket"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	// Type-change-on-update is a store contract (TKT-0C57FS amendment A3):
	// the data-migration rename_entity_type step is a plain UpdateEntity
	// with a new Type, so the record must fully RELOCATE — readable under
	// the new type, gone from the old type's listings, and (on path-keyed
	// backends like fsstore) leaving no orphan record at the old location.
	t.Run("UpdateChangesType", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "v1")
		require.NoError(t, s.CreateEntity(ctx(), e))

		moved := entity.New("T-1", "issue")
		moved.SetString("title", "v1")
		require.NoError(t, s.UpdateEntity(ctx(), moved))

		got, err := s.GetEntity(ctx(), entity.Ref{ID: "T-1"})
		require.NoError(t, err)
		assert.Equal(t, "issue", got.Type)

		oldCount, err := s.CountEntities(ctx(), store.EntityQuery{Type: "ticket", Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 0, oldCount, "old type still lists the entity")
		newCount, err := s.CountEntities(ctx(), store.EntityQuery{Type: "issue", Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 1, newCount, "new type does not list the entity")
	})

	t.Run("Delete", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "Bye")
		require.NoError(t, s.CreateEntity(ctx(), e))

		result, err := s.DeleteFamily(ctx(), "T-1", false)
		require.NoError(t, err)
		require.Len(t, result.DeletedEntities, 1)
		assert.Equal(t, "T-1", result.DeletedEntities[0].ID)

		_, err = s.GetEntity(ctx(), entity.Ref{ID: "T-1"})
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.DeleteFamily(ctx(), "NOPE", false)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteCascadeRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "A", Type: "requires", To: "B"}, nil)
		require.NoError(t, err)

		result, err := s.DeleteFamily(ctx(), "A", true)
		require.NoError(t, err)
		assert.Len(t, result.DeletedRelations, 1)
		assert.Equal(t, "requires", result.DeletedRelations[0].Type)
	})

	t.Run("DeleteCascadeRelationsToSide", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "A", Type: "requires", To: "B"}, nil)
		require.NoError(t, err)

		result, err := s.DeleteFamily(ctx(), "B", true)
		require.NoError(t, err)
		assert.Len(t, result.DeletedRelations, 1)
		assert.Equal(t, "requires", result.DeletedRelations[0].Type)

		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "A", Type: "requires", To: "B"})
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNoCascadeWithRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "A", Type: "requires", To: "B"}, nil)
		require.NoError(t, err)

		_, err = s.DeleteFamily(ctx(), "A", false)
		assert.ErrorIs(t, err, store.ErrHasRelations)

		_, err = s.GetEntity(ctx(), entity.Ref{ID: "A"})
		assert.NoError(t, err)
	})

	t.Run("Rename", func(t *testing.T) {
		s := f(t)
		e := entity.New("OLD-1", "ticket")
		e.SetString("title", "Keep me")
		require.NoError(t, s.CreateEntity(ctx(), e))

		result, err := s.RenameFamily(ctx(), "OLD-1", "NEW-1")
		require.NoError(t, err)
		assert.Equal(t, 0, result.RelationsUpdated)

		_, err = s.GetEntity(ctx(), entity.Ref{ID: "OLD-1"})
		assert.ErrorIs(t, err, store.ErrNotFound)

		got, err := s.GetEntity(ctx(), entity.Ref{ID: "NEW-1"})
		require.NoError(t, err)
		assert.Equal(t, "Keep me", got.GetString("title"))
	})

	t.Run("RenameUpdatesRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), entity.RelationKey{From: "A", Type: "requires", To: "B"}, nil)
		s.CreateRelation(ctx(), entity.RelationKey{From: "C", Type: "blocks", To: "A"}, nil)

		result, err := s.RenameFamily(ctx(), "A", "A2")
		require.NoError(t, err)
		assert.Equal(t, 2, result.RelationsUpdated)

		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "A", Type: "requires", To: "B"})
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "C", Type: "blocks", To: "A"})
		assert.ErrorIs(t, err, store.ErrNotFound)

		r1, err := s.GetRelation(ctx(), entity.RelationKey{From: "A2", Type: "requires", To: "B"})
		require.NoError(t, err)
		assert.Equal(t, "A2", r1.From)

		r2, err := s.GetRelation(ctx(), entity.RelationKey{From: "C", Type: "blocks", To: "A2"})
		require.NoError(t, err)
		assert.Equal(t, "A2", r2.To)
	})

	t.Run("RenameNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.RenameFamily(ctx(), "NOPE", "NEW")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("RenameConflict", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "feature")))

		_, err := s.RenameFamily(ctx(), "A", "B")
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("RenameRejectsDoubleDash", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))

		_, err := s.RenameFamily(ctx(), "A", "B--C")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")
	})

	t.Run("CreateRejectsEmptyID", func(t *testing.T) {
		s := f(t)
		err := s.CreateEntity(ctx(), entity.New("", "feature"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty ID")
	})

	t.Run("ListStableOrder", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"C", "A", "B"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "t")))
		}

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Equal(t, []string{"A", "B", "C"}, ids)
	})

	t.Run("ListEarlyBreak", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
			if len(ids) == 1 {
				break
			}
		}
		assert.Len(t, ids, 1)
	})

	t.Run("CountWithIDs", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))

		n, err := s.CountEntities(ctx(), store.EntityQuery{IDs: []string{"A", "C"}, Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "feature", IDs: []string{"A", "C"}, Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("RenameSkipsUnrelatedRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		s.CreateRelation(ctx(), entity.RelationKey{From: "B", Type: "links", To: "C"}, nil)

		result, err := s.RenameFamily(ctx(), "A", "A2")
		require.NoError(t, err)
		assert.Equal(t, 0, result.RelationsUpdated)

		r, err := s.GetRelation(ctx(), entity.RelationKey{From: "B", Type: "links", To: "C"})
		require.NoError(t, err)
		assert.Equal(t, "B", r.From)
	})

	t.Run("RenameEmitsEvent", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))

		events, cancel := s.Subscribe(10)
		defer cancel()

		_, err := s.RenameFamily(ctx(), "A", "B")
		require.NoError(t, err)

		ev := <-events
		assert.Equal(t, store.EventEntityUpdated, ev.Op)
		assert.Equal(t, "B", ev.EntityID)
		assert.Equal(t, "feature", ev.EntityType)
	})
}
