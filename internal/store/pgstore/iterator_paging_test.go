package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// pagingStore is a store whose iterators read one row per page, so every
// row boundary is a page boundary.
func pagingStore(t *testing.T) (*pgstore.Store, *pgxpool.Pool, context.Context) {
	t.Helper()
	pool := newScopedPool(t)
	st, err := pgstore.New(pool)
	require.NoError(t, err)
	pgstore.SetIteratorPageSizeForTest(t, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	return st, pool, ctx
}

// TestIteratorPaging_LegacyIDAtPageBoundary pins that a row whose id the
// current grammar rejects still resumes paging after it. A resume key that
// was rendered and parsed back would fail to parse there, restart the
// listing, and never end (BUG-9TGOH1 review).
func TestIteratorPaging_LegacyIDAtPageBoundary(t *testing.T) {
	st, pool, ctx := pagingStore(t)
	for _, id := range []string{"LEG-1", "LEG-2", "LEG-3"} {
		mustCreateEntity(t, st, id, "feature")
	}
	_, err := st.CreateRelation(ctx, "LEG-1", "nest", "LEG-3", nil)
	require.NoError(t, err)
	_, err = st.CreateRelation(ctx, "LEG-2", "nest", "LEG-3", nil)
	require.NoError(t, err)
	// "--" is no longer a valid id; rows written before that rule keep it.
	_, err = pool.Exec(ctx, `UPDATE entities SET id = 'LEG--2' WHERE id = 'LEG-2'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE relations SET from_id = 'LEG--2' WHERE from_id = 'LEG-2'`)
	require.NoError(t, err)

	var ids []string
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: "feature"}) {
		require.NoError(t, err)
		ids = append(ids, e.ID)
	}
	require.Equal(t, []string{"LEG--2", "LEG-1", "LEG-3"}, ids)

	var froms []string
	for r, err := range st.ListRelations(ctx, store.RelationQuery{Type: "nest"}) {
		require.NoError(t, err)
		froms = append(froms, r.From)
	}
	require.Equal(t, []string{"LEG--2", "LEG-1"}, froms)
}

// TestIteratorPaging_WorldPrimeChangesMidIteration pins that a world-scoped
// listing yields an entity once even when a write between two pages changes
// which face it resolves to. A keyset on the resolved (id, face) would admit
// the new prime again, because the new face sorts after the old one.
func TestIteratorPaging_WorldPrimeChangesMidIteration(t *testing.T) {
	st, _, ctx := pagingStore(t)
	face := func(name string) entity.Face {
		p, err := entity.ParseFace(name)
		require.NoError(t, err)
		return p
	}
	create := func(id, p string) {
		e := entity.New(id, "page")
		if p != "" {
			e.Face = face(p)
		}
		e.SetString("title", id+" "+p)
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	create("PAGE-1", "published")
	create("PAGE-2", "published")

	world := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: []entity.Face{face("review"), face("published")}, Fallback: store.FallbackExclude},
	})
	var ids []string
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: "page", World: world}) {
		require.NoError(t, err)
		ids = append(ids, e.ID)
		if e.ID == "PAGE-1" {
			// Now PAGE-1 resolves to review, which sorts after published.
			create("PAGE-1", "review")
		}
	}
	require.Equal(t, []string{"PAGE-1", "PAGE-2"}, ids)
}
