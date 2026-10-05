package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunAddressTests pins how every backend answers an [entity.Ref] (TKT-KQXVF7).
// A row is addressed by (id, face). A ref that names no row is ErrNotFound,
// whether the row is absent or the ref is malformed: a malformed face can
// never be declared, so it names nothing, and a backend must not map it onto
// some other row, a path, or a SQL error.
func RunAddressTests(t *testing.T, f Factory) {
	seed := func(t *testing.T, s store.Store, id string, faces ...entity.Face) {
		t.Helper()
		for _, face := range faces {
			e := entity.New(id, "page")
			e.Face = face
			e.SetString("title", string(face))
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
	}

	t.Run("EachFaceIsItsOwnRow", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-1", "draft", "published")
		for _, face := range []entity.Face{"draft", "published"} {
			got, err := s.GetEntity(ctx(), entity.Ref{ID: "DOC-1", Face: face})
			require.NoError(t, err, "face %s", face)
			assert.Equal(t, face, got.Face)
			assert.Equal(t, string(face), got.GetString("title"))
		}
	})

	t.Run("BareRefOnAFacedFamilyIsNotFound", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-2", "draft")
		_, err := s.GetEntity(ctx(), entity.Ref{ID: "DOC-2"})
		assert.ErrorIs(t, err, store.ErrNotFound,
			"a family stored only at named faces has no zero-face row (DEC-NPZICR)")
	})

	t.Run("MalformedRefsAreNotFound", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-3", "", "draft")
		for _, tc := range []struct {
			name string
			ref  entity.Ref
		}{
			{"zero ref", entity.Ref{}},
			{"face only", entity.Ref{Face: "draft"}},
			{"joined address as id", entity.Ref{ID: "DOC-3@draft"}},
			{"path face", entity.Ref{ID: "DOC-3", Face: "../x"}},
			{"face with @", entity.Ref{ID: "DOC-3", Face: "a@b"}},
			{"face with NUL", entity.Ref{ID: "DOC-3", Face: "dr\x00aft"}},
			{"upper-case face", entity.Ref{ID: "DOC-3", Face: "DRAFT"}},
			{"id with NUL", entity.Ref{ID: "DOC-3\x00"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := s.GetEntity(ctx(), tc.ref)
				assert.ErrorIs(t, err, store.ErrNotFound, "GetEntity")
				_, err = s.DeleteFace(ctx(), tc.ref)
				assert.ErrorIs(t, err, store.ErrNotFound, "DeleteFace")
			})
		}
		// Nothing above touched a row.
		for _, face := range []entity.Face{"", "draft"} {
			_, err := s.GetEntity(ctx(), entity.Ref{ID: "DOC-3", Face: face})
			require.NoError(t, err, "face %q survives", face)
		}
	})
}

// RunFamilyTests pins the family operations: DeleteFamily, DeleteFace and
// RenameFamily on a family stored only at named faces.
func RunFamilyTests(t *testing.T, f Factory) {
	seed := func(t *testing.T, s store.Store, id string, faces ...entity.Face) {
		t.Helper()
		for _, face := range faces {
			e := entity.New(id, "page")
			e.Face = face
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
	}

	// RR-2466U1: deleting the last face removes the entity, so no edge may
	// keep pointing at it or starting from it.
	t.Run("LastFaceTakesEveryIncidentEdge", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-10", "draft", "published")
		seed(t, s, "SRC-1", "")
		seed(t, s, "DST-1", "")
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "SRC-1", Type: "links", To: "DOC-10"}, nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(),
			entity.RelationKey{From: "DOC-10", FromFace: "draft", Type: "references", To: "DST-1"}, nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(),
			entity.RelationKey{From: "DOC-10", FromFace: "published", Type: "references", To: "DST-1"}, nil)
		require.NoError(t, err)

		res, err := s.DeleteFace(ctx(), entity.Ref{ID: "DOC-10", Face: "published"})
		require.NoError(t, err)
		assert.Len(t, res.DeletedRelations, 1, "not the last face: only its own edge")
		assert.Len(t, collectRelations(t, s, store.RelationQuery{To: "DOC-10"}), 1,
			"the inbound edge survives while a face remains")

		res, err = s.DeleteFace(ctx(), entity.Ref{ID: "DOC-10", Face: "draft"})
		require.NoError(t, err)
		assert.Len(t, res.DeletedRelations, 2, "the last face takes its edge and the inbound one")
		assert.Empty(t, collectRelations(t, s, store.RelationQuery{EntityID: "DOC-10"}),
			"no edge may outlive the entity")
		fam, err := store.FamilyHeaders(ctx(), s, "DOC-10")
		require.NoError(t, err)
		assert.Empty(t, fam)
	})

	t.Run("ZeroFaceDeleteOfAFacelessType", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-11", "")
		seed(t, s, "SRC-2", "")
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "SRC-2", Type: "links", To: "DOC-11"}, nil)
		require.NoError(t, err)

		res, err := s.DeleteFace(ctx(), entity.Ref{ID: "DOC-11"})
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 1)
		assert.Len(t, res.DeletedRelations, 1, "the only face is the last face")
		assert.Empty(t, collectRelations(t, s, store.RelationQuery{EntityID: "DOC-11"}))
	})

	t.Run("DeleteFamilyOfNamedFacesOnly", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-12", "draft", "published")
		res, err := s.DeleteFamily(ctx(), "DOC-12", true)
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 2)
		_, err = s.DeleteFamily(ctx(), "DOC-12", true)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("RenameFamilyOfNamedFacesOnly", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-13", "draft", "published")
		seed(t, s, "DOC-14", "draft")
		_, err := s.RenameFamily(ctx(), "DOC-13", "DOC-14")
		assert.ErrorIs(t, err, store.ErrConflict, "any face of the new id is a conflict")

		_, err = s.RenameFamily(ctx(), "DOC-13", "DOC-15")
		require.NoError(t, err)
		fam, err := store.FamilyHeaders(ctx(), s, "DOC-15")
		require.NoError(t, err)
		assert.Len(t, fam, 2)
		_, err = s.RenameFamily(ctx(), "DOC-13", "DOC-16")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("UpdateEntityIfAtANamedFace", func(t *testing.T) {
		s := f(t)
		seed(t, s, "DOC-17", "draft", "published")
		read, err := s.GetEntity(ctx(), entity.Ref{ID: "DOC-17", Face: "draft"})
		require.NoError(t, err)
		v := store.VersionOf(read)
		read.SetString("title", "edited")
		_, err = s.UpdateEntityIf(ctx(), read, store.UpdateCondition{ExpectedVersion: v})
		require.NoError(t, err)

		got, err := s.GetEntity(ctx(), entity.Ref{ID: "DOC-17", Face: "draft"})
		require.NoError(t, err)
		assert.Equal(t, "edited", got.GetString("title"))
		pub, err := s.GetEntity(ctx(), entity.Ref{ID: "DOC-17", Face: "published"})
		require.NoError(t, err)
		assert.Empty(t, pub.GetString("title"), "the sibling face is untouched")
	})
}
