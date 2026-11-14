package storetest

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunHeaderPageTests is the conformance suite for store.ListEntityHeadersPage
// (TKT-Q918DFR). A header page is ListEntitiesPage projected onto headers: at
// every step of a walk it holds the same rows in the same order and returns
// the same NextCursor, natively and through the generic fallback, so a
// cursor from either read resumes the other.
//
// The seed has a faced type with every family shape and an unfaced type, all
// with bodies, so the walks cross families, types and the world fallback.
func RunHeaderPageTests(t *testing.T, f Factory) {
	draft, published := mustFace(t, "draft"), mustFace(t, "published")
	def := entity.Face("")
	world := store.NewWorldScope(map[string]store.TypeResolution{
		"doc": {Chain: []entity.Face{published, draft}, Fallback: store.FallbackDefaultState},
	})
	excluding := store.NewWorldScope(map[string]store.TypeResolution{
		"doc": {Chain: []entity.Face{published}, Fallback: store.FallbackExclude},
	})

	seed := func(t *testing.T) store.Store {
		t.Helper()
		s := f(t)
		for i, r := range []struct {
			id   string
			face entity.Face
		}{
			{"DOC-1", def}, {"DOC-1", draft}, {"DOC-1", published},
			{"DOC-2", draft},
			{"DOC-3", def},
			{"DOC-4", draft}, {"DOC-4", published},
		} {
			e := entity.New(r.id, "doc")
			e.Face = r.face
			e.SetString("title", fmt.Sprintf("%s at %q", r.id, r.face))
			e.SetString("rank", strconv.Itoa(i))
			e.Content = "body of " + r.id
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		for _, id := range []string{"NOTE-1", "NOTE-2", "NOTE-3"} {
			e := entity.New(id, "note")
			e.SetString("title", id)
			e.Content = "body of " + id
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		return s
	}

	queries := []struct {
		name string
		q    store.EntityQuery
	}{
		{"default world, every type", store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}},
		{"default world, one type", store.EntityQuery{Type: "note", Faces: store.InWorld(store.TrivialScope())}},
		{"by ids", store.EntityQuery{IDs: []string{"DOC-1", "NOTE-2"}, Faces: store.AllFaces()}},
		{"all faces", store.EntityQuery{Type: "doc", Faces: store.AllFaces()}},
		{"all faces, every type", store.EntityQuery{Faces: store.AllFaces()}},
		{"at faces", store.EntityQuery{Type: "doc", Faces: store.AtFaces(def, published)}},
		{"world with fallback", store.EntityQuery{Type: "doc", Faces: store.InWorld(world)}},
		{"world, every type", store.EntityQuery{Faces: store.InWorld(world)}},
		{"world excluding", store.EntityQuery{Type: "doc", Faces: store.InWorld(excluding)}},
		{"world under a face ceiling", store.EntityQuery{
			Type: "doc", Faces: store.InWorld(world), FaceIn: []entity.Face{draft, def},
		}},
	}

	t.Run("PagesMatchEntityPages", func(t *testing.T) {
		s := seed(t)
		for _, qc := range queries {
			for _, limit := range []int{0, 1, 2, 3, 100} {
				t.Run(fmt.Sprintf("%s/limit %d", qc.name, limit), func(t *testing.T) {
					walkHeaderPages(t, s, qc.q, limit)
				})
			}
		}
	})

	// A walk whose page size changes between calls resumes after the last
	// row read, as ListEntitiesPage does, and a cursor from ListEntitiesPage
	// resumes a header walk.
	t.Run("CursorsResumeAcrossReads", func(t *testing.T) {
		s := seed(t)
		for _, qc := range queries {
			t.Run(qc.name, func(t *testing.T) {
				want := keysOf(t, s.ListEntities(ctx(), qc.q))
				var got []string
				q := qc.q
				q.Limit = 1
				for step := 0; ; step++ {
					require.Less(t, step, 20, "the walk does not end")
					var next string
					if step%2 == 0 {
						page, err := store.ListEntityHeadersPage(ctx(), s, q)
						require.NoError(t, err)
						got = append(got, headerKeys(page.Items)...)
						next = page.NextCursor
					} else {
						page, err := s.ListEntitiesPage(ctx(), q)
						require.NoError(t, err)
						got = append(got, entityKeys(page.Items)...)
						next = page.NextCursor
					}
					if next == "" {
						break
					}
					q.Cursor = next
					q.Limit = min(2*q.Limit, 3)
				}
				assert.Equal(t, want, got)
			})
		}
	})

	t.Run("EmptyStore", func(t *testing.T) {
		s := f(t)
		page, err := store.ListEntityHeadersPage(ctx(), s,
			store.EntityQuery{Limit: 2, Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Empty(t, page.Items)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("InvalidCursorFails", func(t *testing.T) {
		s := seed(t)
		q := store.EntityQuery{Limit: 2, Cursor: "!!not-a-cursor!!", Faces: store.InWorld(store.TrivialScope())}
		_, wantErr := s.ListEntitiesPage(ctx(), q)
		require.Error(t, wantErr, "precondition: the entity page refuses the cursor")
		_, err := store.ListEntityHeadersPage(ctx(), s, q)
		assert.Error(t, err)
	})

	t.Run("MutatingHeaderPropsDoesNotAffectStore", func(t *testing.T) {
		s := seed(t)
		page, err := store.ListEntityHeadersPage(ctx(), s,
			store.EntityQuery{Type: "note", Limit: 1, Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		page.Items[0].Properties["title"] = "mutated"
		got, err := s.GetEntity(ctx(), entity.Ref{ID: "NOTE-1"})
		require.NoError(t, err)
		assert.Equal(t, "NOTE-1", got.GetString("title"))
	})
}

// walkHeaderPages walks q at limit with ListEntitiesPage, the native header
// page and the fallback header page in lockstep, and requires every step to
// agree.
func walkHeaderPages(t *testing.T, s store.Store, q store.EntityQuery, limit int) {
	t.Helper()
	q.Limit = limit
	want := keysOf(t, s.ListEntities(ctx(), q))
	var walked []string
	for step := 0; ; step++ {
		require.Less(t, step, 20, "the walk does not end")
		ents, err := s.ListEntitiesPage(ctx(), q)
		require.NoError(t, err)
		native, err := store.ListEntityHeadersPage(ctx(), s, q)
		require.NoError(t, err)
		fallback, err := store.ListEntityHeadersPage(ctx(), pagerOnly{s}, q)
		require.NoError(t, err)

		projected := make([]store.EntityHeader, 0, len(ents.Items))
		for _, e := range ents.Items {
			projected = append(projected, store.HeaderOf(e))
		}
		assert.Equal(t, projected, native.Items, "step %d: native header page", step)
		assert.Equal(t, projected, fallback.Items, "step %d: fallback header page", step)
		assert.Equal(t, ents.NextCursor, native.NextCursor, "step %d: native cursor", step)
		assert.Equal(t, ents.NextCursor, fallback.NextCursor, "step %d: fallback cursor", step)
		if limit > 0 {
			assert.LessOrEqual(t, len(native.Items), limit, "step %d: page over its limit", step)
		}
		walked = append(walked, headerKeys(native.Items)...)
		if native.NextCursor == "" {
			break
		}
		q.Cursor = native.NextCursor
	}
	assert.Equal(t, want, walked, "the walk yields every row once, in ListEntities order")
}
