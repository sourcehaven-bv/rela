package sqlitestore_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

func allFeatures() store.ElementScope {
	return store.ElementScope{
		Entities:  map[string]store.ElementTypeScope{"feature": {AllowAll: true, Visibility: &store.FieldVisibility{}}},
		Relations: map[string]store.RelationElementScope{"depends-on": {Visibility: &store.FieldVisibility{}}},
	}
}

// A row whose stored element is NULL is pending: counted, never summed or
// listed with a hash.
func TestElementRange_NullElementIsPending(t *testing.T) {
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "e.db"))
	g, err := sqlitestore.NewElementRanges(s)
	require.NoError(t, err)
	ctx := context.Background()
	seedElementRows(t, s)
	_, err = db.DB().Exec(`UPDATE entities SET element_hash = NULL WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = db.DB().Exec(`UPDATE relations SET element_hash = NULL`)
	require.NoError(t, err)

	sum, err := g.RangeFingerprint(ctx, allFeatures(), store.ElementRange{Unbounded: true})
	require.NoError(t, err)
	require.Equal(t, uint64(2), sum.Count)
	require.Equal(t, uint64(1), sum.Pending)
	list, err := g.ListRange(ctx, allFeatures(), store.ElementRange{Unbounded: true}, store.RangeList{Limit: 10})
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].Pending)
	require.Empty(t, list[0].Hash)
	require.False(t, list[1].Pending)
	require.NotEmpty(t, list[1].Hash)

	sum, err = g.RangeFingerprint(ctx, allFeatures(), store.ElementRange{Relations: true, Unbounded: true})
	require.NoError(t, err)
	require.Equal(t, store.RangeSum{Count: 1, Pending: 1}, sum)
}

// A narrow range walks the element index instead of the whole table.
func TestElementRange_ExplainUsesElementIndex(t *testing.T) {
	s := open(t)
	seed(t, s, func(v store.Store) {
		for i := range 500 {
			id := fmt.Sprintf("FEAT-%04d", i)
			mustCreate(t, v, entity.New(id, "feature"))
			if i > 0 {
				mustRelate(t, v, id, "depends-on", fmt.Sprintf("FEAT-%04d", i-1))
			}
		}
	})
	for _, tc := range []struct {
		r     store.ElementRange
		index string
	}{
		{store.ElementRange{Lo: "feature/FEAT-0100", Hi: "feature/FEAT-0120"}, "entities_element_idx"},
		{store.ElementRange{Relations: true, Lo: "FEAT-0100", Hi: "FEAT-0120"}, "relations_element_idx"},
	} {
		plan, err := s.ExplainRangeList(context.Background(), allFeatures(), tc.r, store.RangeList{Limit: 50})
		require.NoError(t, err)
		t.Logf("plan:\n%s", plan)
		require.Contains(t, plan, tc.index, "range %+v does not use its element index", tc.r)
	}
}

// A hidden property name the builder cannot render sends the request to the
// reference, which answers it.
func TestElementRange_UnsafeNameFallsBack(t *testing.T) {
	s := open(t)
	g, err := sqlitestore.NewElementRanges(s)
	require.NoError(t, err)
	ctx := context.Background()
	e := entity.New("FEAT-1", "feature")
	e.Properties[`quo"te`] = "x"
	require.NoError(t, s.CreateEntity(ctx, e))
	require.NoError(t, s.CreateEntity(ctx, entity.New("FEAT-2", "feature")))
	scope := store.ElementScope{Entities: map[string]store.ElementTypeScope{"feature": {
		AllowAll:   true,
		Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{{Property: `quo"te`}}},
	}}}
	sum, err := g.RangeFingerprint(ctx, scope, store.ElementRange{Unbounded: true})
	require.NoError(t, err)
	require.Equal(t, uint64(2), sum.Count)
	require.Equal(t, uint64(1), sum.Pending)
}
