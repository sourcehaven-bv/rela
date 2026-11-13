package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunQueryTests runs entity query conformance tests.
func RunQueryTests(t *testing.T, f Factory) {
	t.Run("ListAll", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 4)
	})

	t.Run("ListByType", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{Type: "feature", Faces: store.InWorld(store.TrivialScope())}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 3)
	})

	t.Run("ListByIDs", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{IDs: []string{"FEAT-001", "REQ-001"}, Faces: store.InWorld(store.TrivialScope())}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 2)
	})

	t.Run("ListByTypeAndIDs", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		q := store.EntityQuery{Type: "feature", IDs: []string{"FEAT-001", "REQ-001"}, Faces: store.InWorld(store.TrivialScope())}
		for e, err := range s.ListEntities(ctx(), q) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 1)
		assert.Contains(t, ids, "FEAT-001")
	})

	t.Run("Count", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		n, err := s.CountEntities(ctx(), store.EntityQuery{Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 4, n)

		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "feature", Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 3, n)

		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "nonexistent", Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("HighestID", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		n, err := s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 13, n)

		n, err = s.HighestID(ctx(), "REQ")
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		n, err = s.HighestID(ctx(), "NOPE")
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	// The SQL backends read HighestID as a range over "<prefix>-" (TKT-KQXVF7),
	// and used to read it as a LIKE pattern. Ids just outside the range, and a
	// prefix holding a LIKE wildcard, must not count.
	t.Run("HighestIDIsExactPrefix", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"FEAT-4", "FEATURE-99", "FEAT_9", "FEA-50", "FEAT", "A_B-4", "AXB-9"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "feature")), id)
		}

		n, err := s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 4, n)

		n, err = s.HighestID(ctx(), "A_B")
		require.NoError(t, err)
		assert.Equal(t, 4, n, "_ in the prefix is a literal, not a wildcard")
	})

	// Ids are case-insensitive identities, so a hand-written id in any case
	// holds its number (BUG-RLBXTG5). A backend that compared the prefix
	// bytewise missed "Req-9", and the next mint could collide with it.
	t.Run("HighestIDFoldsPrefixCase", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"REQ-4", "Req-9", "rEQ-7"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "requirement")), id)
		}
		for _, prefix := range []string{"REQ", "req", "Req"} {
			n, err := s.HighestID(ctx(), prefix)
			require.NoError(t, err)
			assert.Equal(t, 9, n, "prefix %q", prefix)
		}
	})

	// Every backend parses the suffix the same way: digits only. Some read a
	// leading digit run, so "FEAT-50x" counted as 50 on them and not on
	// others (BUG-RLBXTG5).
	t.Run("HighestIDCountsDigitsOnly", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"FEAT-007", "FEAT-50x", "FEAT-60-1", "FEAT-SUB-70", "FEAT80"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "feature")), id)
		}
		n, err := s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 7, n)
	})
}
