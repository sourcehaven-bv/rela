package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunValidationTests runs input validation conformance tests.
func RunValidationTests(t *testing.T, f Factory) {
	t.Run("CreateEntityRejectsInvalidIDs", func(t *testing.T) {
		s := f(t)

		invalid := []struct {
			id   string
			want string
		}{
			{"", "empty"},
			{"foo/bar", "path separator"},
			{"foo\\bar", "path separator"},
			{"foo\x00bar", "control character"},
			{"foo\nbar", "control character"},
			{"foo\tbar", "control character"},
			{"foo\x7fbar", "control character"},
			{"a--b", "consecutive dashes"},
		}
		for _, tc := range invalid {
			err := s.CreateEntity(ctx(), entity.New(tc.id, "t"))
			assert.Errorf(t, err, "id %q should be rejected", tc.id)
			if err != nil {
				assert.Containsf(t, err.Error(), tc.want,
					"error for id %q should mention %q", tc.id, tc.want)
			}
		}
	})

	t.Run("RelationKeyRejectsDoubleDash", func(t *testing.T) {
		s := f(t)

		err := s.CreateEntity(ctx(), entity.New("A--B", "t"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")

		require.NoError(t, s.CreateEntity(ctx(), entity.New("A-B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C-D", "t")))

		_, err = s.CreateRelation(ctx(), "A-B", "req--ires", "C-D", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")

		_, err = s.CreateRelation(ctx(), "A-B", "requires", "C-D", nil)
		require.NoError(t, err)
	})

	t.Run("RenameKeyCollapseDeterministic", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("X", "t")))

		_, err := s.CreateRelation(ctx(), "A", "requires", "X",
			&store.RelationData{Content: "from-A"})
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "C", "requires", "X",
			&store.RelationData{Content: "from-C"})
		require.NoError(t, err)

		_, err = s.RenameEntity(ctx(), "A", "C")
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	// Case-variant IDs are ONE identity, in every backend (BUG-3RCWNS).
	//
	// This lives in the shared conformance suite rather than in fsstore's own
	// tests on purpose. fsstore on a case-insensitive filesystem (macOS,
	// Windows) folds "abc" and "ABC" onto one file, while memstore and pgstore
	// (id TEXT COLLATE "C") keep them as two rows. Entities move between
	// backends via migration and `rela sync`, so a project holding both would
	// silently lose one on import. The backends must agree on identity, and
	// only a shared test can enforce that.
	//
	// Note this cannot be satisfied by a filesystem accident: the byte-exact
	// backends must actively fold case to pass.
	t.Run("CreateRejectsCaseVariantID", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("abc", "t")))

		err := s.CreateEntity(ctx(), entity.New("ABC", "t"))
		assert.ErrorIsf(t, err, store.ErrConflict,
			"creating \"ABC\" while \"abc\" exists must conflict, not silently overwrite")

		// The original must be intact and still reachable under its own ID.
		got, err := s.GetEntity(ctx(), "abc")
		require.NoError(t, err)
		assert.Equal(t, "abc", got.ID)
	})

	t.Run("RenameRejectsCaseVariantID", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("abc", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("other", "t")))

		_, err := s.RenameEntity(ctx(), "other", "ABC")
		assert.ErrorIsf(t, err, store.ErrConflict,
			"renaming to \"ABC\" while \"abc\" exists must conflict")
	})

	// Renaming an entity to a different casing OF ITSELF is legitimate
	// (abc -> ABC is one entity changing its own display casing), so the
	// case-folded conflict check must not reject it as a self-collision.
	t.Run("RenameToOwnCaseVariantIsAllowed", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("abc", "t")))

		_, err := s.RenameEntity(ctx(), "abc", "ABC")
		require.NoError(t, err, "an entity may change its own casing")

		got, err := s.GetEntity(ctx(), "ABC")
		require.NoError(t, err)
		assert.Equal(t, "ABC", got.ID)
	})
}
