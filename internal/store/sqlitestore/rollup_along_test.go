package sqlitestore_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// alongTestQuery walks child_of up from srcs to projects for depth levels.
func alongTestQuery(depth int, srcs ...string) store.RollupQuery {
	open := store.PropLeaf(store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "on_hold", Scalar: true})
	return store.RollupQuery{Sources: srcs, Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: depth,
		Segments: []store.AlongSegment{{Relation: "child_of", Repeat: true, Through: map[string]store.AlongNode{
			"project": {Gate: store.GraphQuery{EntityType: "project", Faces: store.InWorld(store.TrivialScope())},
				Counts: &open},
		}}},
	}}
}

// The level statement anti-joins the seen triples through the temporary
// table's key, not by scanning it.
func TestAlongLevelExplainSeenUsesKey(t *testing.T) {
	s := open(t)
	plan, err := s.ExplainAlongLevelForTest(context.Background(), alongTestQuery(2, "ACT-1"))
	require.NoError(t, err)
	t.Logf("plan:\n%s", plan)
	require.Contains(t, plan, "SEARCH x USING PRIMARY KEY (src=? AND id=? AND state=?)")
	require.NotContains(t, plan, "SCAN x")
}

// A roll-up past its timeout fails, and the caller leaves it absent.
func TestRollupTimeout(t *testing.T) {
	s := open(t)
	seed(t, s, func(v store.Store) {
		mustCreate(t, v, entity.New("ACT-1", "action"))
		for i := range 30 {
			id := fmt.Sprintf("PRJ-%02d", i)
			mustCreate(t, v, entity.New(id, "project"))
			mustRelate(t, v, "ACT-1", "child_of", id)
		}
	})
	_, err := s.Rollup(context.Background(), alongTestQuery(4, "ACT-1"))
	require.NoError(t, err)
	sqlitestore.SetRollupTimeoutForTest(t, time.Nanosecond)
	_, err = s.Rollup(context.Background(), alongTestQuery(4, "ACT-1"))
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// A list query with a roll-up leaf past the timeout fails closed: the
// leaf's batched Rollup call is bounded, so the scan errors and yields no
// row and the count errors.
func TestRollupLeafQueryTimeout(t *testing.T) {
	s := open(t)
	seed(t, s, func(v store.Store) {
		mustCreate(t, v, entity.New("ACT-1", "action"))
		mustCreate(t, v, entity.New("PRJ-1", "project"))
		mustRelate(t, v, "ACT-1", "child_of", "PRJ-1")
	})
	leaf := store.RollupLeaf(alongTestQuery(4), false)
	q := store.GraphQuery{EntityType: "action", Faces: store.InWorld(store.TrivialScope()), Where: &leaf}
	sqlitestore.SetRollupTimeoutForTest(t, time.Nanosecond)
	var rows int
	var scanErr error
	for _, err := range s.GraphQueryHeaders(context.Background(), q) {
		if err != nil {
			scanErr = err
			break
		}
		rows++
	}
	require.ErrorIs(t, scanErr, context.DeadlineExceeded)
	require.Zero(t, rows)
	_, err := s.CountMatched(context.Background(), q)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
