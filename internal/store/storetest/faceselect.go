package storetest

import (
	"context"
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunFaceSelectionTests is the face-selection conformance suite (TKT-KQXVF7):
// the zero selection is refused by every read, AtFaces is a set (empty
// matches nothing), AllFaces pages resume inside a family, FaceIn composes
// with every mode, and an endpoint match reads its endpoint at its own
// selection or the query's (design A6).
//
// The seed is one faced type with every family shape: all three faces
// (DOC-1), a named face only (DOC-2) and the default face only (DOC-3), plus
// an unfaced type whose identity edges point at them.
func RunFaceSelectionTests(t *testing.T, f Factory) {
	draft, published := mustFace(t, "draft"), mustFace(t, "published")
	def := entity.Face("")
	draftWorld := store.NewWorldScope(map[string]store.TypeResolution{
		"doc": {Chain: []entity.Face{draft}, Fallback: store.FallbackDefaultState},
	})

	seed := func(t *testing.T) store.Store {
		t.Helper()
		s := f(t)
		for _, r := range []struct {
			id     string
			face   entity.Face
			status string
		}{
			{"DOC-1", def, "closed"}, {"DOC-1", draft, "open"}, {"DOC-1", published, "closed"},
			{"DOC-2", draft, "open"},
			{"DOC-3", def, "open"},
		} {
			e := entity.New(r.id, "doc")
			e.Face = r.face
			e.SetString("status", r.status)
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		for i, doc := range []string{"DOC-1", "DOC-2", "DOC-3"} {
			own := entity.New([]string{"OWN-1", "OWN-2", "OWN-3"}[i], "owner")
			require.NoError(t, s.CreateEntity(ctx(), own))
			_, err := s.CreateRelation(ctx(), own.ID, "owns", doc, nil)
			require.NoError(t, err)
		}
		return s
	}

	t.Run("ZeroSelectionIsInvalid", func(t *testing.T) {
		s := seed(t)
		eq := store.EntityQuery{Type: "doc"}
		gq := store.GraphQuery{EntityType: "doc"}

		assert.ErrorIs(t, firstErr(s.ListEntities(ctx(), eq)), store.ErrInvalidQuery, "ListEntities")
		_, err := s.ListEntitiesPage(ctx(), eq)
		assert.ErrorIs(t, err, store.ErrInvalidQuery, "ListEntitiesPage")
		_, err = s.CountEntities(ctx(), eq)
		assert.ErrorIs(t, err, store.ErrInvalidQuery, "CountEntities")
		assert.ErrorIs(t, firstErr(store.ListEntityHeaders(ctx(), s, eq)), store.ErrInvalidQuery,
			"ListEntityHeaders, native where the backend has it")
		assert.ErrorIs(t, firstErr(store.ListEntityHeaders(ctx(), listerOnly{s}, eq)), store.ErrInvalidQuery,
			"ListEntityHeaders, generic fallback")

		assert.ErrorIs(t, firstErr(s.GraphQuery(ctx(), gq)), store.ErrInvalidQuery, "GraphQuery")
		assert.ErrorIs(t, firstErr(store.GraphQueryHeaders(ctx(), s, gq)), store.ErrInvalidQuery, "GraphQueryHeaders")
		_, _, err = s.GraphCount(ctx(), gq)
		assert.ErrorIs(t, err, store.ErrInvalidQuery, "GraphCount")
		_, err = s.MatchingIDs(ctx(), gq, []string{"DOC-1"})
		assert.ErrorIs(t, err, store.ErrInvalidQuery, "MatchingIDs")
	})

	// rows reads the same selection through every entity and graph read and
	// requires them to agree, so each case below is asserted once.
	rows := func(t *testing.T, s store.Store, sel store.FaceSelection, faceIn []entity.Face) []string {
		t.Helper()
		eq := store.EntityQuery{Type: "doc", Faces: sel, FaceIn: faceIn}
		list := keysOf(t, s.ListEntities(ctx(), eq))
		n, err := s.CountEntities(ctx(), eq)
		require.NoError(t, err)
		assert.Len(t, list, n, "CountEntities disagrees with ListEntities")
		page, err := s.ListEntitiesPage(ctx(), eq)
		require.NoError(t, err)
		assert.Equal(t, list, entityKeys(page.Items), "ListEntitiesPage")
		heads := []string{}
		for h, err := range store.ListEntityHeaders(ctx(), s, eq) {
			require.NoError(t, err)
			heads = append(heads, h.ID+"@"+h.Face.String())
		}
		assert.Equal(t, list, heads, "ListEntityHeaders")

		gq := store.GraphQuery{EntityType: "doc", Faces: sel, FaceIn: faceIn}
		assert.Equal(t, list, keysOf(t, s.GraphQuery(ctx(), gq)), "GraphQuery")
		matched, _, err := s.GraphCount(ctx(), gq)
		require.NoError(t, err)
		assert.Equal(t, len(list), matched, "GraphCount")
		return list
	}

	t.Run("AtFaces", func(t *testing.T) {
		s := seed(t)
		assert.Empty(t, rows(t, s, store.AtFaces(), nil), "an empty face set matches nothing")
		assert.Equal(t, []string{"DOC-1@", "DOC-1@draft", "DOC-2@draft", "DOC-3@"},
			rows(t, s, store.AtFaces(def, draft), nil), "exactly the rows at the listed faces")
		assert.Equal(t, []string{"DOC-1@published"}, rows(t, s, store.AtFaces(published), nil))
	})

	t.Run("AllFacesPagesResumeMidFamily", func(t *testing.T) {
		s := seed(t)
		all := rows(t, s, store.AllFaces(), nil)
		require.Len(t, all, 5)
		var paged []string
		q := store.EntityQuery{Type: "doc", Faces: store.AllFaces(), Limit: 2}
		for range 5 {
			page, err := s.ListEntitiesPage(ctx(), q)
			require.NoError(t, err)
			paged = append(paged, entityKeys(page.Items)...)
			if page.NextCursor == "" {
				break
			}
			q.Cursor = page.NextCursor
		}
		assert.Equal(t, all, paged, "the first page ends inside DOC-1's family and the next resumes there")
	})

	t.Run("FaceInComposesWithEveryMode", func(t *testing.T) {
		s := seed(t)
		for _, tc := range []struct {
			name   string
			sel    store.FaceSelection
			faceIn []entity.Face
			want   []string
		}{
			{"default world, ceiling excludes its face", store.InWorld(store.DefaultWorld()),
				[]entity.Face{draft}, nil},
			{"default world, ceiling admits its face", store.InWorld(store.DefaultWorld()),
				[]entity.Face{def}, []string{"DOC-1@", "DOC-3@"}},
			{"draft world falls back under the ceiling", store.InWorld(draftWorld),
				[]entity.Face{def}, []string{"DOC-1@", "DOC-3@"}},
			{"draft world, ceiling admits draft", store.InWorld(draftWorld),
				[]entity.Face{draft, def}, []string{"DOC-1@draft", "DOC-2@draft", "DOC-3@"}},
			{"all faces", store.AllFaces(),
				[]entity.Face{draft}, []string{"DOC-1@draft", "DOC-2@draft"}},
			{"face set intersected", store.AtFaces(def, draft),
				[]entity.Face{draft, published}, []string{"DOC-1@draft", "DOC-2@draft"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := rows(t, s, tc.sel, tc.faceIn)
				if len(tc.want) == 0 {
					assert.Empty(t, got)
					return
				}
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("EndpointSelection", func(t *testing.T) {
		s := seed(t)
		owners := []string{"OWN-1", "OWN-2", "OWN-3"}
		for _, tc := range []struct {
			name     string
			query    store.FaceSelection
			endpoint store.FaceSelection // zero: inherit the query's
			want     []string
		}{
			{"inherits the default world", store.InWorld(store.DefaultWorld()), store.FaceSelection{},
				[]string{"OWN-3"}},
			{"inherits AllFaces: any face of the endpoint", store.AllFaces(), store.FaceSelection{},
				owners},
			{"inherits AtFaces", store.AtFaces(def), store.FaceSelection{},
				[]string{"OWN-3"}},
			{"own AllFaces overrides the query's world", store.InWorld(store.DefaultWorld()), store.AllFaces(),
				owners},
			{"own AtFaces: an endpoint without the face does not match", store.AllFaces(), store.AtFaces(published),
				nil},
			{"own world ranks the endpoint", store.AllFaces(), store.InWorld(draftWorld),
				owners},
			{"own default world overrides AllFaces", store.AllFaces(), store.InWorld(store.DefaultWorld()),
				[]string{"OWN-3"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				q := store.GraphQuery{EntityType: "owner", Faces: tc.query, HasOutbound: &store.RelationPredicate{
					OfTypes: []string{"owns"},
					EndpointMatch: &store.EndpointPredicate{
						EntityType: "doc",
						Faces:      tc.endpoint,
						Props:      []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
					},
				}}
				var got []string
				for e, err := range s.GraphQuery(ctx(), q) {
					require.NoError(t, err)
					got = append(got, e.ID)
				}
				slices.Sort(got)
				assert.Equal(t, tc.want, got, "GraphQuery")
				ids, err := s.MatchingIDs(ctx(), q, owners)
				require.NoError(t, err)
				for _, id := range owners {
					assert.Equal(t, slices.Contains(tc.want, id), ids[id], "MatchingIDs[%s]", id)
				}
			})
		}
	})
}

func mustFace(t *testing.T, name string) entity.Face {
	t.Helper()
	f, err := entity.ParseFace(name)
	require.NoError(t, err)
	return f
}

// listerOnly hides every optional capability of a store, so
// [store.ListEntityHeaders] takes its generic fallback.
type listerOnly struct{ s store.EntityLister }

func (l listerOnly) ListEntities(c context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return l.s.ListEntities(c, q)
}

func firstErr[T any](seq iter.Seq2[T, error]) error {
	for _, err := range seq {
		if err != nil {
			return err
		}
	}
	return nil
}

func keysOf(t *testing.T, seq iter.Seq2[*entity.Entity, error]) []string {
	t.Helper()
	es, err := drainEntities(seq)
	require.NoError(t, err)
	return entityKeys(es)
}
