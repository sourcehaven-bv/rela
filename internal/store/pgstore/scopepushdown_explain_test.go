//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
func TestExplainPushedTraversalScopeListUsesDerivedIndexes(t *testing.T) {
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
  feature: {label: Feature, id_prefix: FEAT, properties: {status: {type: string}}}
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
		status := "common"
		if i == features-1 {
			status = "rare"
		}
		e := entity.New(fmt.Sprintf("FEAT-%06d", i), "feature")
		e.Properties["status"] = status
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
		_, err = s.CreateRelation(ctx, id, "implements", fmt.Sprintf("FEAT-%06d", i%features), nil)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)

	def, _ := meta.GetEntityDef("ticket")
	prog, err := predicatefns.NewEvaluator(meta).CompileWithCurrentUser("ticket", def.QueryScopes["open-rare"])
	require.NoError(t, err)
	lowered, ok := queryplan.LowerScope(prog, meta, "ticket", "")
	require.True(t, ok, "the scope must lower")
	require.Len(t, lowered.Traversals, 1)
	spec := lowered.Traversals[0]
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
			Property: name, Op: store.PropEqual, Value: spec.Props[name].(predicate.String).String(), Scalar: true,
		})
	}
	pred, err := acl.UngatedTraversal(hop)
	require.NoError(t, err)

	q := store.GraphQuery{
		EntityType: "ticket",
		Props:      lowered.Props,
		Related:    []store.DirectedRelation{{Incoming: hop.Incoming, Pred: *pred}},
		OrderBy:    []store.OrderSpec{{Property: "title"}},
		Limit:      50,
	}
	for _, countOnly := range []bool{false, true} {
		sqlText, args := pgstore.BuildGraphQuerySQLForTest(q, countOnly)
		rows, err := pool.Query(ctx, "EXPLAIN "+sqlText, args...)
		require.NoError(t, err)
		var lines []string
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			lines = append(lines, line)
		}
		rows.Close()
		require.NoError(t, rows.Err())
		plan := strings.Join(lines, "\n")
		t.Logf("countOnly=%v plan:\n%s", countOnly, plan)
		for _, table := range []string{"entities", "relations"} {
			if strings.Contains(plan, "Seq Scan on "+table) {
				t.Errorf("countOnly=%v: the pushed scope scans %s; a lowered scope "+
					"must be answered from indexes:\n%s", countOnly, table, plan)
			}
		}
	}
}
