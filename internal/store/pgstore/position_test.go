//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A position is ONE statement whatever the size of the ordered set
// (TKT-U9DYW4): the rows are numbered inside the database and only the
// target's row leaves it.
func TestGraphPosition_OneStatementAtAnySize(t *testing.T) {
	st := openWriter(t, freshFeedSchema(t))
	seeded := 0
	for _, size := range []int{5, 60} {
		for ; seeded < size; seeded++ {
			e := entity.New(fmt.Sprintf("T-%03d", seeded), "ticket")
			e.SetString("due", fmt.Sprintf("2026-01-%02d", 28-seeded%28))
			require.NoError(t, st.CreateEntity(context.Background(), e))
		}
		ctx, stats := store.WithQueryStats(context.Background())
		q := store.GraphQuery{EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "due"}}}
		pos, found, err := store.GraphPosition(ctx, st, q, "T-003")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, size, pos.Total)
		require.EqualValues(t, 1, stats.Queries(), "position at %d rows", size)
	}
}
