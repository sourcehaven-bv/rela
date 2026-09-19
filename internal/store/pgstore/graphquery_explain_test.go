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
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
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
// selectivity estimates favor them. Only the per-iteration CTE
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
			_, err = s.CreateRelation(ctx, "engineering", "owns", id, nil)
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
	// Either access method is fine; both are index-backed.
	indexed := strings.Contains(plan, "Index Scan using relations_type_from_idx") ||
		strings.Contains(plan, "Bitmap Index Scan on relations_type_from_idx")
	if !indexed {
		t.Errorf("composite index relations_type_from_idx is not used in the plan; "+
			"the CTE will degrade per-iteration:\n%s", plan)
	}
}

func TestGraphQueryExplainUsesDerivedStaticQueryIndex(t *testing.T) {
	const n = 5000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	spec := []store.DerivedObjectSpec{{
		Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"status"},
	}}
	_, err = s.Reconcile(ctx, spec, store.ReconcileOptions{})
	require.NoError(t, err)

	for i := range n {
		status := "closed"
		if i == n-1 {
			status = "open"
		}
		e := entity.New(fmt.Sprintf("TASK-%06d", i), "task")
		e.Properties["status"] = status
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	_, err = pool.Exec(ctx, "ANALYZE entities")
	require.NoError(t, err)

	plan := explainGraphQuery(t, pool, store.GraphQuery{
		EntityType: "task",
		Props: []store.PropPredicate{{
			Property: "status", Op: store.PropEqual, Value: "open", Scalar: true,
		}},
	})
	t.Logf("plan:\n%s", plan)
	if !strings.Contains(plan, "rela_derived_query__") {
		t.Fatalf("derived static-query index is not used:\n%s", plan)
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

// A pushed list page — filter on one property, order by another, LIMIT —
// scans the derived list index instead of sorting the type (TKT-1U8XYN).
func TestGraphQueryExplainPagedListUsesDerivedListIndex(t *testing.T) {
	const n = 5000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	spec := []store.DerivedObjectSpec{{
		Kind: store.DerivedListIndex, Type: "task", Properties: []string{"status"}, OrderBy: []string{"due"},
	}}
	_, err = s.Reconcile(ctx, spec, store.ReconcileOptions{})
	require.NoError(t, err)

	for i := range n {
		e := entity.New(fmt.Sprintf("TASK-%06d", i), "task")
		e.Properties["status"] = []string{"open", "done"}[i%2]
		e.Properties["due"] = fmt.Sprintf("2026-%02d-%02d", 1+i%12, 1+i%28)
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	_, err = pool.Exec(ctx, "ANALYZE entities")
	require.NoError(t, err)

	plan := explainGraphQuery(t, pool, store.GraphQuery{
		EntityType: "task",
		Props:      []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
		OrderBy:    []store.OrderSpec{{Property: "due"}},
		Limit:      25,
	})
	t.Logf("plan:\n%s", plan)
	if !strings.Contains(plan, "rela_derived_list__") {
		t.Fatalf("derived list index is not used:\n%s", plan)
	}
	if strings.Contains(plan, "Sort") {
		t.Fatalf("page still sorts instead of walking the index:\n%s", plan)
	}
}

// TestEndpointMatchExplainUsesDerivedIndex pins the finding that made the
// propCond alias refactor mandatory (TKT-RELTRV).
//
// A derived query index is PARTIAL:
//
//	CREATE INDEX ... ON entities ((properties->>'status'))
//	  WHERE type = 'concept' AND jsonb_typeof(properties->'status') = 'string';
//
// PostgreSQL matches a partial index only when the query IMPLIES its
// predicate. An endpoint filter spelled as a bare `->>` comparison is correct
// and silently unindexed: measured at 5k concepts it bitmap-scanned every row
// of the type to find 10. Emitting the same jsonb_typeof guard the scalar
// spelling carries makes the index a candidate and the scan disappears.
//
// This asserts the SHAPE (the index is used), not a timing, so it is stable in
// CI. It fails if a future change hand-rolls the endpoint comparison instead of
// routing it through propCondOn.
func TestEndpointMatchExplainUsesDerivedIndex(t *testing.T) {
	const tickets = 5000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	// The index must exist on the TRAVERSED-TO type, not the queried type.
	// Reconcile the spec queryplan DERIVES for this traversal rather than a
	// hand-written one, so the test proves the derivation and the lowering
	// agree — a derivation that produced the wrong type or property would
	// still create an index, and a hand-written spec would hide that.
	specs := queryplan.TraversalIndexSpecs(
		traversalExplainProgram(t), traversalExplainMeta(), "ticket")
	require.Len(t, specs, 1, "queryplan must derive exactly one traversal index spec")
	require.Equal(t, "concept", specs[0].Type)
	_, err = s.Reconcile(ctx, specs, store.ReconcileOptions{})
	require.NoError(t, err)

	// 500 concepts, exactly one of them 'rare'; every ticket points at one.
	const concepts = 500
	for i := range concepts {
		status := "active"
		if i == concepts-1 {
			status = "rare"
		}
		e := entity.New(fmt.Sprintf("CON-%06d", i), "concept")
		e.Properties["status"] = status
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	for i := range tickets {
		id := fmt.Sprintf("TKT-%06d", i)
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "ticket")))
		_, err = s.CreateRelation(ctx, id, "caused-by", fmt.Sprintf("CON-%06d", i%concepts), nil)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)

	plan := explainGraphQuery(t, pool, store.GraphQuery{
		EntityType: "ticket",
		HasOutbound: &store.RelationPredicate{
			OfTypes: []string{"caused-by"},
			EndpointMatch: &store.EndpointPredicate{
				EntityType: "concept",
				Props: []store.PropPredicate{{
					Property: "status", Op: store.PropEqual, Value: "rare", Scalar: true,
				}},
			},
		},
	})
	t.Logf("plan:\n%s", plan)
	if !strings.Contains(plan, "rela_derived_query__") {
		t.Fatalf("endpoint-match filter does not reach the derived index on the "+
			"traversed-to type; the join degrades to a scan of every row of that "+
			"type:\n%s", plan)
	}
}

// traversalExplainMeta / traversalExplainProgram describe the same traversal
// the EXPLAIN below runs, so the derived spec and the executed query cannot
// drift apart.
func traversalExplainMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"concept_status": {Values: []string{"active", "rare"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Properties: map[string]metamodel.PropertyDef{}},
			"concept": {Properties: map[string]metamodel.PropertyDef{"status": {Type: "concept_status"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"caused-by": {From: []string{"ticket"}, To: []string{"concept"}},
		},
	}
}

func traversalExplainProgram(t *testing.T) *predicate.Program {
	t.Helper()
	env := predicate.NewEnv()
	require.NoError(t, env.DeclareVar("entity", predicate.RecordType{}))
	prog, err := predicate.Compile(env, `related(entity, 'caused-by', { status = 'rare' })`)
	require.NoError(t, err)
	return prog
}
