package elementnaive_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func TestNew_RejectsNil(t *testing.T) {
	_, err := elementnaive.New(nil)
	require.Error(t, err)
}

// The reference costs the same number of store reads at any number of rows
// (collection-reads budget).
func TestRanges_ReadsDoNotGrowWithRows(t *testing.T) {
	ctx := context.Background()
	scope := store.ElementScope{
		Entities: map[string]store.ElementTypeScope{
			"ticket": {
				Query: &store.GraphQuery{Props: []store.PropPredicate{{
					Property: "level", Op: store.PropEqual, Value: "public", Scalar: true,
				}}},
				Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{{
					Property: "secret",
					Visible: store.RowCondition{Any: []*store.GraphQuery{{Props: []store.PropPredicate{{
						Property: "show", Op: store.PropEqual, Value: "yes", Scalar: true,
					}}}}},
				}}},
			},
			"epic": {AllowAll: true, Visibility: &store.FieldVisibility{}},
		},
		Relations: map[string]store.RelationElementScope{"tracks": {Visibility: &store.FieldVisibility{}}},
	}
	base := memstore.New()
	c := storetest.NewCounting(base)
	g, err := elementnaive.New(c)
	require.NoError(t, err)

	var calls []map[string]int
	seeded := 0
	for _, size := range []int{10, 50} {
		for ; seeded < size; seeded++ {
			e := entity.New(fmt.Sprintf("T-%03d", seeded), "ticket")
			e.SetString("level", []string{"public", "private"}[seeded%2])
			e.SetString("secret", "s")
			require.NoError(t, base.CreateEntity(ctx, e))
			epic := fmt.Sprintf("E-%03d", seeded)
			require.NoError(t, base.CreateEntity(ctx, entity.New(epic, "epic")))
			_, err := base.CreateRelation(ctx, entity.RelationKey{From: epic, Type: "tracks", To: e.ID}, nil)
			require.NoError(t, err)
		}
		c.Reset()
		_, err := g.RangeFingerprint(ctx, scope, store.ElementRange{Unbounded: true})
		require.NoError(t, err)
		_, err = g.ListRange(ctx, scope, store.ElementRange{Relations: true, Unbounded: true}, store.RangeList{Limit: 10})
		require.NoError(t, err)
		calls = append(calls, c.Calls())
	}
	require.Equal(t, calls[0], calls[1], "store reads at 10 and at 50 rows")
}
