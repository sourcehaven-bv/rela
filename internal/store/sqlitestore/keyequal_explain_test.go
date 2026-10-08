package sqlitestore_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// AC6 (TKT-SM20FG): an external-ref lookup is one SQL statement on sqlite,
// scoped by the type index, never the naive fallback.
func TestGraphQueryKeyEqualIsOneQuery(t *testing.T) {
	q := store.GraphQuery{
		EntityType: "ticket",
		Props:      []store.PropPredicate{{Property: "basecamp", Op: store.PropKeyEqual, Key: "id", Value: "42"}},
		Faces:      store.AllFaces(),
	}
	require.True(t, sqlitestore.GraphQueryPushesDownForTest(q))

	s := open(t)
	seed(t, s, func(v store.Store) {
		for i := range 500 {
			e := entity.New(fmt.Sprintf("T-%04d", i), "ticket")
			e.Properties["basecamp"] = map[string]any{"id": strconv.Itoa(i)}
			mustCreate(t, v, e)
		}
	})
	plan := explain(t, s, q)
	require.Contains(t, plan, "type=?", "the lookup is not scoped by the type index")
}
