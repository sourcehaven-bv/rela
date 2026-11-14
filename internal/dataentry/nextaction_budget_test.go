package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// nextActionQueryBudget is the store reads of one next-action resolve with
// one query source matching every ticket: one window of candidates, and the
// two membership reads that resolve the principal's roles.
const nextActionQueryBudget = 3

// nextActionQueryOp resolves a query source that matches every ticket,
// padding included, so a read of the whole match grows with the store.
var nextActionQueryOp budgetOp = func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
	t.Helper()
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "work"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "work", Query: "type:ticket", Suggest: "{title} needs a look"},
		})
	got, code := getNextAction(gateCtxFor(ctx, t, d), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "tickets", got.Suggestion.Source)
}

// TestNextActionBudget_QuerySourceReadsAWindow pins TKT-CKLZMWB: a query
// source reads the candidates the engine keeps, not every match. At scale 5
// the postgres scale run read 28 keyset pages of critical tasks per resolve,
// of which the engine kept 20.
func TestNextActionBudget_QuerySourceReadsAWindow(t *testing.T) {
	c, detail := readsFor(t, nextActionQueryOp)
	assertBudget(t, "next-action query source", c, nextActionQueryBudget, detail)
}

// seedNumberedTickets seeds n tickets TKT-0001.. with status(i) as status.
func seedNumberedTickets(app *App, n int, status func(i int) string) {
	for i := 1; i <= n; i++ {
		seedEntity(app, &entity.Entity{
			ID: fmt.Sprintf("TKT-%04d", i), Type: "ticket",
			Properties: map[string]any{"title": fmt.Sprintf("Ticket %d", i), "status": status(i)},
		})
	}
}

// The suggestion on screen stays on screen when it falls past the window
// the source reads, which happens when candidates are added before it.
func TestNextAction_CurrentPastTheWindowStays(t *testing.T) {
	app := newTestAppV1(t)
	n := 3 * nextaction.DefaultCandidateCap
	seedNumberedTickets(app, n, func(int) string { return "open" })
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "work"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "work", Query: "type:ticket", Suggest: "{title}"},
		})
	current := fmt.Sprintf("TKT-%04d", n)
	got, code := getNextActionAt(aliceCtx(), t, app,
		"/api/v1/_next_action?current_source=tickets&current_entity="+current)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, current, got.Suggestion.EntityID)
}

// A filter the store does not apply drops rows after the read, so a window
// may hold no candidates while later rows match. The read goes on until it
// finds them.
func TestNextAction_WindowSkipsRowsTheGoFilterDrops(t *testing.T) {
	app := newTestAppV1(t)
	n := 3 * nextaction.DefaultCandidateCap
	// Only the last three tickets are open. `!=` is not pushed into the
	// store, so the first windows hold only done tickets.
	seedNumberedTickets(app, n, func(i int) string {
		if i > n-3 {
			return "open"
		}
		return "done"
	})
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "work"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "work", Query: "type:ticket prop:status!=done", Suggest: "{title}"},
		})
	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Contains(t, []string{
		fmt.Sprintf("TKT-%04d", n-2), fmt.Sprintf("TKT-%04d", n-1), fmt.Sprintf("TKT-%04d", n),
	}, got.Suggestion.EntityID)
}

// A suggestion on screen that no longer matches costs no second read when
// the first read was not cut short: a sorted query is read whole already.
func TestNextAction_StaleCurrentOnAWholeReadReadsOnce(t *testing.T) {
	app, counting, d, ctx := newBudgetAppPadded(t, 30, 0, memstore.New(), budgetMeta())
	ctx = gateCtxFor(ctx, t, d)
	const query = "type:ticket sort:title"

	counting.Reset()
	_, err := app.queries.executeQuery(ctx, query)
	require.NoError(t, err)
	once := counting.Cost().Calls

	counting.Reset()
	got, err := app.queryCandidates(ctx, query, nil,
		nextaction.Want{First: nextaction.DefaultCandidateCap, Keep: "TKT-GONE"})
	require.NoError(t, err)
	require.Len(t, got, 30)
	require.Equal(t, once, counting.Cost().Calls, counting.String())
}

func entityIDs(es []*entity.Entity) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

// TestFirstN_SameLeadingRowsAsAFullRead pins the claim firstNByTypes makes:
// a read of the first n results returns the first n rows of the full read,
// in the same order, under a role reached through a member-of walk.
func TestFirstN_SameLeadingRowsAsAFullRead(t *testing.T) {
	const tickets = 60 // odd tickets are done, even tickets open
	app, _, d, ctx := newBudgetAppPadded(t, tickets, 0, memstore.New(), budgetMeta())
	ctx = gateCtxFor(ctx, t, d)

	for _, tc := range []struct {
		name  string
		query string
		n     int
	}{
		{"prefix of one type", "type:ticket", 20},
		{"filter pushed into the store", "type:ticket prop:status=open", 20},
		{"filter applied in Go only", "type:ticket prop:status!=done", 20},
		{"first type runs out", "type:feature,ticket", 8},
		{"fewer matches than n", "type:feature", 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			full, err := app.queries.executeQuery(ctx, tc.query)
			require.NoError(t, err)
			got, cut, err := app.queries.executeQueryCut(ctx, tc.query, queryOptions{firstN: tc.n})
			require.NoError(t, err)

			want := full[:min(tc.n, len(full))]
			require.NotEmpty(t, want)
			require.Equal(t, entityIDs(want), entityIDs(got))
			require.Equal(t, len(full) >= tc.n, cut, "cut reports that the read stopped at n")
		})
	}
}

// TestFirstN_SparseMatchReadsBoundedWindows pins the cost of a match the
// store cannot narrow: growing windows, then the rest of the type in one
// read, rather than one read per window to the end of the type.
func TestFirstN_SparseMatchReadsBoundedWindows(t *testing.T) {
	const tickets = 500 // every ticket is open or done, so nothing matches
	app, counting, d, ctx := newBudgetAppPadded(t, tickets, 0, memstore.New(), budgetMeta())
	ctx = gateCtxFor(ctx, t, d)
	counting.Reset()

	got, cut, err := app.queries.executeQueryCut(ctx,
		"type:ticket prop:status!=open prop:status!=done", queryOptions{firstN: nextaction.DefaultCandidateCap})
	require.NoError(t, err)
	require.Empty(t, got)
	require.False(t, cut)
	// Windows of 20, 80 and 320 rows, then the remaining 80 in one read.
	require.Equal(t, maxQueryWindows+1, counting.Calls()["GraphQueryHeaders"], counting.String())
}
