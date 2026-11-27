//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestExplainPushedTraversalScopeListUsesDerivedIndexes pins the cost of a
// list page whose query scope lowers to the store (TKT-XKCNCL). The scope
// pairs an equality with a traversal, so the page and its count carry a
// [store.GraphQuery.Related] entry beside the pushed equality. Neither may
// scan the ticket type: the equality must reach the list index and the
// traversal's endpoint filter the index derived on the far-end type.
//
// The equalities come from [queryplan.LowerScope], the hop from
// [predicatefns.ResolveTraversal] and the index set from
// [queryplan.StaticIndexSpecs], as in production. The hop is lowered with
// [acl.UngatedTraversal] rather than a principal's gate, which adds a row
// gate to the endpoint but does not change the join shape planned here.
//
// The endpoint filter runs both as a plain equality and, with 'rare' as the
// schema default, as the [store.PropEqualOrEmpty] that relresolve.Hop
// lowers it to. Some features leave status unset, so the empty arm has rows
// to find, and a traversal-only page must read them from the derived
// empty-value index rather than every feature.
func TestExplainPushedTraversalScopeListUsesDerivedIndexes(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		op           store.PropOp
	}{
		{"equal", "{type: string}", store.PropEqual},
		{"equal_or_empty", "{type: string, default: rare}", store.PropEqualOrEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) { explainPushedTraversalScope(t, tc.status, tc.op) })
	}
}

func explainPushedTraversalScope(t *testing.T, featureStatus string, op store.PropOp) {
	t.Helper()
	const tickets, features = 5000, 500
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	meta, err := metamodel.Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties: {title: {type: string}, status: {type: string}}
    query_scopes:
      open-rare: "entity.status == 'open' and related(entity, 'implements', { status = 'rare' })"
  feature: {label: Feature, id_prefix: FEAT, properties: {status: ` + featureStatus + `}}
relations:
  implements: {label: implements, from: [ticket], to: [feature]}
`))
	require.NoError(t, err)
	cfg := &dataentryconfig.Config{Lists: map[string]dataentryconfig.List{
		"tickets": {
			EntityType: "ticket", QueryScope: "open-rare",
			Sort: []dataentryconfig.SortSpec{{Property: "title"}},
		},
	}}
	_, err = s.Reconcile(ctx, queryplan.StaticIndexSpecs(cfg, meta), store.ReconcileOptions{})
	require.NoError(t, err)

	// One ticket in a thousand is open and one feature in 500 is rare, so a
	// scan of either type stands out from an index probe.
	for i := range features {
		e := entity.New(fmt.Sprintf("FEAT-%06d", i), "feature")
		switch {
		case i == features-1:
			e.Properties["status"] = "rare"
		case i%100 != 0:
			e.Properties["status"] = "common"
		}
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	for i := range tickets {
		status := "closed"
		if i%1000 == 0 {
			status = "open"
		}
		id := fmt.Sprintf("TKT-%06d", i)
		e := entity.New(id, "ticket")
		e.Properties["status"] = status
		e.Properties["title"] = fmt.Sprintf("Ticket %06d", i)
		require.NoError(t, s.CreateEntity(ctx, e))
		_, err = s.CreateRelation(ctx, entity.RelationKey{From: id, Type: "implements", To: fmt.Sprintf("FEAT-%06d", i%features)}, nil)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)

	def, _ := meta.GetEntityDef("ticket")
	prog, err := predicatefns.NewEvaluator(meta).CompileWithCurrentUser("ticket", def.QueryScopes["open-rare"])
	require.NoError(t, err)
	lowered, ok := queryplan.LowerCondition(prog, meta, "ticket", "")
	require.True(t, ok, "the scope must lower")
	require.Len(t, lowered.Traversals, 1)
	spec := lowered.Traversals[0].Spec
	resolved, err := predicatefns.ResolveTraversal(meta, "ticket", spec)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	hop := acl.TraversalHop{
		RelationTypes: []string{resolved[0].Relation},
		Incoming:      resolved[0].Incoming,
		EntityType:    resolved[0].Target,
	}
	for _, name := range spec.PropNames() {
		hop.Props = append(hop.Props, store.PropPredicate{
			Property: name, Op: op, Value: spec.Props[name].(predicate.String).String(), Scalar: true,
		})
	}
	pred, err := acl.UngatedTraversal(hop)
	require.NoError(t, err)

	for _, withEquality := range []bool{true, false} {
		q := store.GraphQuery{
			EntityType: "ticket",
			Related:    []store.DirectedRelation{{Incoming: hop.Incoming, Pred: *pred}},
			OrderBy:    []store.OrderSpec{{Property: "title"}},
			Limit:      50,
			Faces:      store.InWorld(store.TrivialScope()),
		}
		if withEquality {
			q.Props = lowered.Props
		}
		requireIndexedPlans(ctx, t, pool, q)
		if !withEquality && op == store.PropEqualOrEmpty {
			sqlText, args := pgstore.BuildGraphQuerySQLForTest(q, false)
			plan := explainPlan(ctx, t, pool, sqlText, args)
			require.Contains(t, plan, "rela_derived_empty__",
				"the empty arm must be read from the derived empty-value index")
		}
	}
}

func explainPlan(ctx context.Context, t *testing.T, pool *pgxpool.Pool, sqlText string, args []any) string {
	t.Helper()
	rows, err := pool.Query(ctx, "EXPLAIN "+sqlText, args...)
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

func requireIndexedPlans(ctx context.Context, t *testing.T, pool *pgxpool.Pool, q store.GraphQuery) {
	t.Helper()
	for _, countOnly := range []bool{false, true} {
		sqlText, args := pgstore.BuildGraphQuerySQLForTest(q, countOnly)
		plan := explainPlan(ctx, t, pool, sqlText, args)
		t.Logf("countOnly=%v plan:\n%s", countOnly, plan)
		for _, table := range []string{"entities", "relations"} {
			if strings.Contains(plan, "Seq Scan on "+table) {
				t.Errorf("countOnly=%v: the pushed scope scans %s; a lowered scope "+
					"must be answered from indexes:\n%s", countOnly, table, plan)
			}
		}
	}
}
