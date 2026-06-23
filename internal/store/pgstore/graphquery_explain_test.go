//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestGraphQueryExplainUsesIndex pins the CTE planning shape: the
// recursive-CTE iteration MUST use the composite (rel_type, from_id)
// index introduced in migration 0002. The CTE iterates once per
// expansion step; without the composite, each step falls back to a
// seq scan over `relations`, which is the quadratic foot-cannon the
// migration exists to prevent.
//
// The assertion is index-positive (the index name appears in the
// plan) rather than seq-scan-negative — the planner is allowed to
// pick seq scans for the outer SELECT at small scale when its
// selectivity estimates favour them. Only the per-iteration CTE
// recursion needs to be index-backed; that's where blast radius is
// non-linear in graph size.
//
// Run with:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres \
//	  -run=TestGraphQueryExplainUsesIndex -v ./internal/store/pgstore/...
func TestGraphQueryExplainUsesIndex(t *testing.T) {
	const n = 5000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	// Seed a typical ACL-shaped graph: 1 principal in 1 group with N
	// candidate entities, ~10% owned by the group.
	require.NoError(t, s.CreateEntity(ctx, entity.New("alice", "person")))
	require.NoError(t, s.CreateEntity(ctx, entity.New("engineering", "team")))
	_, err = s.CreateRelation(ctx, "alice", "member-of", "engineering", nil)
	require.NoError(t, err)
	for i := range n {
		id := fmt.Sprintf("TKT-%06d", i)
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "ticket")))
		if i%10 == 0 {
			_, err := s.CreateRelation(ctx, "engineering", "owns", id, nil)
			require.NoError(t, err)
		}
	}

	// ANALYZE so the planner has stats — without it, low-cardinality
	// heuristics can mask a missing index.
	_, err = pool.Exec(ctx, "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)

	q := store.GraphQuery{
		EntityType: "ticket",
		HasInbound: &store.RelationPredicate{
			Endpoints:      []string{"alice"},
			OfTypes:        []string{"owns"},
			InheritThrough: []string{"member-of"},
			Depth:          5,
		},
	}

	plan := explainGraphQuery(t, pool, q)
	t.Logf("plan:\n%s", plan)

	// The recursive CTE must use the composite index for the
	// per-iteration rel_type lookup.
	if !strings.Contains(plan, "Index Scan using relations_type_from_idx") &&
		!strings.Contains(plan, "Bitmap Index Scan on relations_type_from_idx") {
		t.Errorf("composite index relations_type_from_idx is not used in the plan; the CTE will degrade per-iteration:\n%s", plan)
	}
}

// explainGraphQuery runs EXPLAIN (no ANALYZE — we only need the plan
// shape) against the SQL pgstore would build for q, and returns the
// formatted plan as a single string.
func explainGraphQuery(t *testing.T, pool *pgxpool.Pool, q store.GraphQuery) string {
	t.Helper()
	sqlText, args := pgstore.BuildGraphQuerySQLForTest(q, false)
	rows, err := pool.Query(context.Background(), "EXPLAIN "+sqlText, args...)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}
