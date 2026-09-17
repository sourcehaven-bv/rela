//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestExplainScopedListUsesDerivedIndex is AC11's proof (TKT-EVR2TU).
//
// The unit tests in internal/queryplan assert the derivation produces the
// right COLUMNS. They cannot tell you the resulting index actually serves the
// query, and AC11's failure mode is invisible to every other kind of test: no
// error, no wrong rows, just a sequential scan on a page the operator was
// promised would be indexed. EXPLAIN is the only thing that distinguishes
// "derived an index" from "derived the RIGHT index".
//
// The list names no scope, so it inherits the type's `default` — the exact
// case where the extra conjunct arrives without appearing anywhere in the
// list's own declaration, which is why the derivation could plausibly miss it.
//
// Run with:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres \
//	  -run=TestExplainScopedListUsesDerivedIndex ./internal/store/pgstore/...
func TestExplainScopedListUsesDerivedIndex(t *testing.T) {
	const n = 5000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Label:      "Taak",
				IDPrefixes: []string{"TAAK-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
				// The default every page of this type inherits.
				QueryScopes: map[string]string{"default": "entity.status == 'open'"},
			},
		},
	}
	// Deliberately names NO query_scope: the inherited default is the case
	// under test.
	cfg := &dataentryconfig.Config{Lists: map[string]dataentryconfig.List{
		"taken": {EntityType: "taak", Sort: []dataentryconfig.SortSpec{{Property: "title"}}},
	}}

	specs := queryplan.StaticIndexSpecs(cfg, meta)
	require.NotEmpty(t, specs, "no index derived for the scoped list")
	_, err = s.Reconcile(ctx, specs, store.ReconcileOptions{})
	require.NoError(t, err)

	// One row in a thousand is open, so a scan is clearly distinguishable
	// from an index probe in the plan.
	for i := range n {
		status := "closed"
		if i%1000 == 0 {
			status = "open"
		}
		e := entity.New(fmt.Sprintf("TAAK-%06d", i), "taak")
		e.Properties["status"] = status
		e.Properties["title"] = fmt.Sprintf("Taak %06d", i)
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	_, err = pool.Exec(ctx, "ANALYZE entities")
	require.NoError(t, err)

	// The query the list's page actually issues: the scope's pushed equality,
	// ordered by the list's sort key.
	plan := explainGraphQuery(t, pool, store.GraphQuery{
		EntityType: "taak",
		Props: []store.PropPredicate{{
			Property: "status", Op: store.PropEqual, Value: "open", Scalar: true,
		}},
		OrderBy: []store.OrderSpec{{Property: "title"}},
		Limit:   50,
	})
	t.Logf("plan:\n%s", plan)

	if !strings.Contains(plan, "rela_derived_list__") {
		t.Fatalf("the scoped list's page does not use its derived index, so every "+
			"page of this type scans. This is AC11's failure mode: nothing errors, "+
			"the rows are correct, the query is just slow.\n%s", plan)
	}
}
