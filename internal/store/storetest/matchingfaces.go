package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunMatchingFacesTests pins [store.GraphQueryer.MatchingFaces], the per-face
// row verdict the ACL read gate runs (acl.Request.ReadableFacesMany). The
// gate asks it under AllFaces with the grant's face allowlist, so a backend
// that answered per id instead of per face row, or dropped a branch's face
// set, would widen or narrow every bare-id read at once.
func RunMatchingFacesTests(t *testing.T, f Factory) {
	face := func(t *testing.T, v string) entity.Face {
		t.Helper()
		p, err := entity.ParseFace(v)
		require.NoError(t, err)
		return p
	}
	seed := func(t *testing.T, s store.Store, id, typ, fc string) {
		t.Helper()
		e := entity.New(id, typ)
		if fc != "" {
			e.Face = face(t, fc)
		}
		e.SetString("title", id+" "+fc)
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
	relate := func(t *testing.T, s store.Store, from, typ, to string) {
		t.Helper()
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: from, Type: typ, To: to}, nil)
		require.NoError(t, err)
	}

	t.Run("AllFacesReturnsEveryMatchingFaceSorted", func(t *testing.T) {
		s := f(t)
		seed(t, s, "P-1", "page", "published")
		seed(t, s, "P-1", "page", "draft")
		seed(t, s, "P-2", "page", "draft")
		seed(t, s, "N-1", "note", "")

		q := store.GraphQuery{EntityType: "page", Faces: store.AllFaces()}
		got, err := s.MatchingFaces(ctx(), q, []string{"P-1", "P-2", "P-9", "N-1"})
		require.NoError(t, err)
		assert.Equal(t, map[string][]entity.Face{
			"P-1": {face(t, "draft"), face(t, "published")},
			"P-2": {face(t, "draft")},
		}, got, "an absent id is no match; another type's id is no match")

		empty, err := s.MatchingFaces(ctx(), q, nil)
		require.NoError(t, err)
		assert.Empty(t, empty)

		q.FaceIn = []entity.Face{face(t, "published")}
		got, err = s.MatchingFaces(ctx(), q, []string{"P-1", "P-2"})
		require.NoError(t, err)
		assert.Equal(t, map[string][]entity.Face{"P-1": {face(t, "published")}}, got,
			"FaceIn trims the faces; P-2 has no published face")
	})

	t.Run("InWorldReturnsOnlyThePrime", func(t *testing.T) {
		s := f(t)
		seed(t, s, "P-1", "page", "published")
		seed(t, s, "P-1", "page", "draft")
		world := store.NewWorldScope(map[string]store.TypeResolution{
			"page": {Chain: []entity.Face{face(t, "draft"), face(t, "published")}, Fallback: store.FallbackExclude},
		})
		got, err := s.MatchingFaces(ctx(), store.GraphQuery{EntityType: "page", Faces: store.InWorld(world)},
			[]string{"P-1"})
		require.NoError(t, err)
		assert.Equal(t, map[string][]entity.Face{"P-1": {face(t, "draft")}}, got)
	})

	// S-11: a conferred role grants faces per relation (store.GraphQuery.Any).
	// Under AllFaces each face row is judged by the branches that reach it,
	// so the owner's draft grant is not laundered onto the published face
	// through the reviewer's edge, or the other way round.
	t.Run("AnyBranchFacesPerRowUnderAllFaces", func(t *testing.T) {
		s := f(t)
		seed(t, s, "P-1", "page", "draft")
		seed(t, s, "P-1", "page", "published")
		for _, u := range []string{"U-owner", "U-reviewer", "U-both", "U-none"} {
			seed(t, s, u, "user", "")
		}
		relate(t, s, "U-owner", "owns", "P-1")
		relate(t, s, "U-reviewer", "reviews", "P-1")
		relate(t, s, "U-both", "owns", "P-1")
		relate(t, s, "U-both", "reviews", "P-1")

		draft, published := face(t, "draft"), face(t, "published")
		for _, tc := range []struct {
			who  string
			want []entity.Face
		}{
			{"U-owner", []entity.Face{draft}},
			{"U-reviewer", []entity.Face{published}},
			{"U-both", []entity.Face{draft, published}},
			{"U-none", nil},
		} {
			t.Run(tc.who, func(t *testing.T) {
				q := store.GraphQuery{
					EntityType: "page",
					Faces:      store.AllFaces(),
					Any: []store.GraphBranch{
						{HasInbound: &store.RelationPredicate{Endpoints: []string{tc.who}, OfTypes: []string{"owns"}},
							FaceIn: []entity.Face{draft}},
						{HasInbound: &store.RelationPredicate{Endpoints: []string{tc.who}, OfTypes: []string{"reviews"}},
							FaceIn: []entity.Face{published}},
					},
				}
				got, err := s.MatchingFaces(ctx(), q, []string{"P-1"})
				require.NoError(t, err)
				assert.Equal(t, tc.want, got["P-1"])

				// The type-level allowlist composes with the branches.
				q.FaceIn = []entity.Face{published}
				got, err = s.MatchingFaces(ctx(), q, []string{"P-1"})
				require.NoError(t, err)
				var want []entity.Face
				if len(tc.want) > 0 && tc.want[len(tc.want)-1] == published {
					want = []entity.Face{published}
				}
				assert.Equal(t, want, got["P-1"], "FaceIn AND the branch face set")
			})
		}
	})
}
