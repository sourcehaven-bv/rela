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

// A list sorted on an enum must page through its derived index rather than
// sorting the whole type, and it must keep doing so once PostgreSQL switches
// from a custom to a generic plan (TKT-9OFGH4).
//
// # Why the generic plan matters
//
// pgx prepares statements, and PostgreSQL re-plans a prepared statement
// generically after five executions. An expression index is matched by
// expression EQUIVALENCE, so a rank built from bind parameters matches a
// literal-valued index only while the planner is still substituting values —
// that is, only for the first five executions. Measured on 200k rows: 4
// buffers on a custom plan, 1,915 once generic, from the identical statement.
//
// # What this test can and cannot prove
//
// `plan_cache_mode` only governs CACHED plans, and a one-shot EXPLAIN over the
// extended protocol has no cache entry — PostgreSQL substitutes the parameters
// at plan time and finds the index even for an expression that a truly generic
// plan would miss. Verified directly: the same statement issued via
// PREPARE/EXECUTE under force_generic_plan seq-scans when the property name is
// bound (Sort + Seq Scan) and index-scans when it is interpolated.
//
// So this test proves the index EXISTS, is shaped correctly, and serves the
// query in both directions. What it cannot prove is the bind-vs-literal
// property, because the harness cannot reach the generic-plan path. That half
// is pinned by TestOrderSQL_RankedKeyInterpolatesPropertyName in
// ordersql_test.go, which asserts on the generated SQL — deterministic, and
// the layer where the mistake is actually made.
//
// Run with:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres \
//	  -run=TestEnumOrderExplain -v ./internal/store/pgstore/...
func TestEnumOrderExplainUsesDerivedIndexUnderGenericPlan(t *testing.T) {
	const n = 4000
	declared := []string{"backlog", "ready", "in-progress", "review", "done"}

	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	spec := []store.DerivedObjectSpec{{
		Kind:        store.DerivedListIndex,
		Type:        "ticket",
		OrderBy:     []string{"status"},
		OrderValues: [][]string{declared},
	}}
	_, err = s.Reconcile(ctx, spec, store.ReconcileOptions{})
	require.NoError(t, err)

	for i := range n {
		e := entity.New(fmt.Sprintf("TKT-%06d", i), "ticket")
		e.Properties = map[string]any{"status": declared[i%len(declared)]}
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	// Without stats the planner's low-cardinality heuristics can pick an
	// index for the wrong reason, or a seq scan despite a usable one.
	_, err = pool.Exec(ctx, "ANALYZE entities")
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		desc bool
	}{
		{"ascending", false},
		{"descending", true}, // one index serves both, via a backward scan
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := explainGeneric(t, pool, store.GraphQuery{
				EntityType: "ticket",
				OrderBy:    []store.OrderSpec{{Property: "status", Values: declared, Descending: tc.desc}},
				Limit:      25,
			})
			t.Logf("plan:\n%s", plan)

			if !strings.Contains(plan, "rela_derived_list__") {
				t.Errorf("enum-ordered page did not use its derived index under a generic plan; "+
					"this is the shape that silently degrades in production:\n%s", plan)
			}
			if strings.Contains(plan, "Seq Scan") {
				t.Errorf("enum-ordered page fell back to a sequential scan:\n%s", plan)
			}
		})
	}
}

// explainGeneric plans the store's own SQL for q with
// plan_cache_mode=force_generic_plan, so bind-parameter substitution cannot
// rescue an expression that does not match its index.
//
// The setting is session-scoped and the pool may hand out another connection
// later, so it is set and reset around the EXPLAIN on one acquired connection
// rather than left on.
func explainGeneric(t *testing.T, pool *pgxpool.Pool, q store.GraphQuery) string {
	t.Helper()
	ctx := context.Background()
	sqlText, args := pgstore.BuildGraphQuerySQLForTest(q, false)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET plan_cache_mode = force_generic_plan")
	require.NoError(t, err)
	defer func() {
		_, resetErr := conn.Exec(ctx, "RESET plan_cache_mode")
		require.NoError(t, resetErr)
	}()

	rows, err := conn.Query(ctx, "EXPLAIN "+sqlText, args...)
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
