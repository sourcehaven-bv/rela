package storetest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunRetiredIDTests checks what [Capabilities.RetiredIDs] declares
// (TKT-J6GISC3). With it, a rename retires the old id: the id resolves to
// its successor and stays reserved. Without it, the store keeps no record
// and the old id is free again; the mention suite checks that such a store
// rewrites the text instead.
func RunRetiredIDTests(t *testing.T, f Factory, declared bool) {
	newEntity := func(id string) *entity.Entity {
		e := entity.New(id, "feature")
		e.SetString("title", id)
		return e
	}
	rename := func(t *testing.T, s store.Store, from, to string) {
		t.Helper()
		_, err := s.RenameFamily(ctx(), from, to)
		require.NoError(t, err)
	}

	t.Run("DeclaredMatchesImplementation", func(t *testing.T) {
		_, implements := f(t).(store.RetiredIDs)
		assert.Equal(t, declared, implements,
			"Capabilities.RetiredIDs must match whether the store implements store.RetiredIDs")
	})

	if !declared {
		t.Run("OldIDIsFreeAgain", func(t *testing.T) {
			s := f(t)
			require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
			rename(t, s, "FEAT-001", "FEAT-002")
			require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		})
		return
	}

	successors := func(t *testing.T, s store.Store, ids ...string) map[string]string {
		t.Helper()
		retired, ok := s.(store.RetiredIDs)
		require.True(t, ok)
		got, err := retired.Successors(ctx(), ids)
		require.NoError(t, err)
		return got
	}

	t.Run("RenameRetiresTheOldID", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-005")))
		rename(t, s, "FEAT-001", "FEAT-002")
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-002"},
			successors(t, s, "FEAT-001", "FEAT-002", "FEAT-005", "NOPE-1"))
		assert.Empty(t, successors(t, s))
	})

	t.Run("RetiredIDIsReserved", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-003")))
		rename(t, s, "FEAT-001", "FEAT-002")
		assert.ErrorIs(t, s.CreateEntity(ctx(), newEntity("FEAT-001")), store.ErrConflict)
		_, err := s.RenameFamily(ctx(), "FEAT-003", "FEAT-001")
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("ChainsCollapse", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		rename(t, s, "FEAT-001", "FEAT-002")
		rename(t, s, "FEAT-002", "FEAT-003")
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-003", "FEAT-002": "FEAT-003"},
			successors(t, s, "FEAT-001", "FEAT-002", "FEAT-003"))
	})

	t.Run("RenameBackReclaimsOwnID", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		rename(t, s, "FEAT-001", "FEAT-002")
		rename(t, s, "FEAT-002", "FEAT-001")
		assert.Equal(t, map[string]string{"FEAT-002": "FEAT-001"},
			successors(t, s, "FEAT-001", "FEAT-002"))
	})

	t.Run("DeleteKeepsTheReservation", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		rename(t, s, "FEAT-001", "FEAT-002")
		_, err := s.DeleteFamily(ctx(), "FEAT-002", true)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-002"}, successors(t, s, "FEAT-001"))
		assert.ErrorIs(t, s.CreateEntity(ctx(), newEntity("FEAT-001")), store.ErrConflict)
		assert.ErrorIs(t, s.CreateEntity(ctx(), newEntity("feat-002")), store.ErrConflict,
			"a successor stays reserved, so no unrelated entity inherits its mentions")
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-003")))
		_, err = s.RenameFamily(ctx(), "FEAT-003", "FEAT-002")
		assert.ErrorIs(t, err, store.ErrConflict)
		n, err := s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		_, err = s.DeleteFamily(ctx(), "FEAT-003", true)
		require.NoError(t, err)
		n, err = s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 2, n, "the generator must not mint a successor id")
	})

	t.Run("HighestIDCountsRetiredIDs", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-009")))
		rename(t, s, "FEAT-009", "OTHER-1")
		n, err := s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 9, n, "the generator must not mint a reserved id")
	})

	t.Run("CaseOnlyRenameRetiresNothing", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("feat-001")))
		rename(t, s, "feat-001", "FEAT-001")
		assert.Empty(t, successors(t, s, "feat-001", "FEAT-001"))
		rename(t, s, "FEAT-001", "FEAT-002")
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-002"},
			successors(t, s, "feat-001", "FEAT-001"))
	})

	t.Run("CaseOnlyRenameRepointsSuccessors", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		rename(t, s, "FEAT-001", "FEAT-002")
		rename(t, s, "FEAT-002", "feat-002")
		assert.Equal(t, map[string]string{"FEAT-001": "feat-002"}, successors(t, s, "FEAT-001"))
		rename(t, s, "feat-002", "FEAT-003")
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-003", "feat-002": "FEAT-003"},
			successors(t, s, "FEAT-001", "feat-002"))
	})

	t.Run("RolledBackRenameRetiresNothing", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		rename(t, s, "FEAT-001", "FEAT-002")
		errAbort := errors.New("abort")
		err := s.Tx(ctx(), func(tx store.Store) error {
			if _, err := tx.RenameFamily(ctx(), "FEAT-002", "FEAT-003"); err != nil {
				return err
			}
			return errAbort
		})
		require.ErrorIs(t, err, errAbort)
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-002"},
			successors(t, s, "FEAT-001", "FEAT-002", "FEAT-003"))
	})

	t.Run("InsideTx", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), newEntity("FEAT-001")))
		require.NoError(t, s.Tx(ctx(), func(tx store.Store) error {
			_, err := tx.RenameFamily(ctx(), "FEAT-001", "FEAT-002")
			return err
		}))
		assert.Equal(t, map[string]string{"FEAT-001": "FEAT-002"}, successors(t, s, "FEAT-001"))
	})
}
