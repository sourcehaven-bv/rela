//go:build postgres

package dataentry

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The gantt reads every hierarchy edge of the chart in one statement per
// relation type, whatever the size of the chart (TKT-K7EM7Z5). The edge read
// sends every node id; at the fixed page size it took one statement per 500
// edges, each carrying all ids, so it grew with the square of the chart.
// TestGanttEdges_CostIsScaleFree pins the rows; this pins the statements.
func TestGanttEdges_OneStatementPerRelationType_Postgres(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	st := openPGStore(t, dsn)
	ctx := context.Background()
	// More edges than one 500-row iterator page holds.
	const total = 1200
	for i := range total {
		key := entity.RelationKey{From: fmt.Sprintf("PRJ-%04d", i), Type: "contains", To: fmt.Sprintf("TSK-%04d", i)}
		_, err := st.CreateRelation(ctx, key, &store.RelationData{})
		require.NoError(t, err)
	}
	h := &ganttHandler{store: st}
	g := dataentryconfig.Gantt{Hierarchy: []string{"contains"}}
	for _, n := range []int{100, total} {
		nodes := map[string]*ganttNode{}
		for i := range n {
			for _, id := range []string{fmt.Sprintf("PRJ-%04d", i), fmt.Sprintf("TSK-%04d", i)} {
				nodes[id] = &ganttNode{id: id}
			}
		}
		qctx, stats := store.WithQueryStats(ctx)
		edges, gerr := h.ganttEdges(qctx, &metamodel.Metamodel{}, g, nodes)
		require.Nil(t, gerr)
		require.Len(t, edges["contains"], n)
		require.EqualValues(t, 1, stats.Queries(), "statements for %d edges", n)
	}
}
