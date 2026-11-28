package storetest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// capHubRows is the number of hidden rows each hub holds, far over the
// hop cap of the tests below.
const capHubRows = 500

// relationRows is the relation rows c's reads returned since its last
// Reset.
func relationRows(c *Counting) int {
	c.counts.mu.Lock()
	defer c.counts.mu.Unlock()
	return c.counts.rows["ListRelations"]
}

// TestRollupGenericSiblingsHubBounded: two hubs of hidden rows, one a
// node's parents and one a parent's children, each far over a hop cap of
// ten. The generic fold reads at most two reads of cap + 1 rows per source
// per step, and both hub sources fail closed.
func TestRollupGenericSiblingsHubBounded(t *testing.T) {
	t.Parallel()
	c := NewCounting(memstore.New())
	q := capSiblingsHopSeed(t, c, capHubRows)
	q.Siblings.HopCap = 10
	c.Reset()
	got, err := store.Rollup(ctx(), c, q)
	require.NoError(t, err)
	require.Equal(t, map[string]store.RollupResult{
		"ACT-1": {OverCap: true}, "ACT-3": {OverCap: true}, "ACT-9": behind(1, "PRJ-2"),
	}, got)
	require.LessOrEqual(t, relationRows(c), 2*2*len(q.Sources)*(q.Siblings.HopCap+1), c.String())
}

// TestRollupGenericAlongHubBounded: PRJ-1 has capHubRows hidden children
// and PRJ-2 one visible one. Under a hop cap of ten the generic fold reads
// at most two reads of cap + 1 rows per source per hop, PRJ-1 fails closed
// and PRJ-2 keeps its answer.
func TestRollupGenericAlongHubBounded(t *testing.T) {
	t.Parallel()
	c := NewCounting(memstore.New())
	alongSeed(t, c, "project", "active", "PRJ-1", "PRJ-2")
	alongSeed(t, c, "action", "open", "ACT-2")
	mustRel(t, c, "ACT-2", "child_of", "PRJ-2")
	for i := range capHubRows {
		id := fmt.Sprintf("ACT-S%03d", i)
		capSecret(t, c, "action", "open", id)
		mustRel(t, c, id, "child_of", "PRJ-1")
	}
	q := capGateAlong(store.RollupQuery{Sources: []string{"PRJ-1", "PRJ-2"}, Func: store.RollupCount,
		Along: &store.AlongWalk{MaxDepth: 2, HopCap: 10, Segments: []store.AlongSegment{{
			Relation: "child_of", SourceIsTo: true, Repeat: true, Through: map[string]store.AlongNode{
				"action": alongNode("action", alongWorld, new(alongStatus("open"))),
			},
		}}}})
	c.Reset()
	got, err := store.Rollup(ctx(), c, q)
	require.NoError(t, err)
	require.Equal(t, map[string]store.RollupResult{"PRJ-1": {OverCap: true}, "PRJ-2": {Count: 1}}, got)
	require.LessOrEqual(t, relationRows(c), 2*q.Along.MaxDepth*len(q.Sources)*(q.Along.HopCap+1), c.String())
}
