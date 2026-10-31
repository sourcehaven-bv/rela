package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Pinned read counts for the sidebar's per-principal endpoints under
// [useFieldPolicy]. Measured, not derived; none moves with row count, which
// is the point. Before the prime (TKT-YLG695) the nav status case with a list
// condition read 134 times at 10 rows and 664 at 50, and nav items 104 and
// 504.
const (
	navStatusFieldPolicyBudget         = 8
	navStatusRuleOnlyFieldPolicyBudget = 7
	// navItemsBudget (querybudget_test.go) measures the same endpoint with
	// the default resolver, which redacts nothing; these redact through the
	// field policy. With a list condition listPage primes the whole
	// population and navItemsFor primes the page again, a fixed cost.
	navItemsFieldPolicyBudget          = 8
	navItemsConditionFieldPolicyBudget = 12
)

// fieldPolicy is [realisticBudgetPolicy] with `visible:` blocks on ticket:
// `editor` (held everywhere) sees the title, and `lead` (conferred by a
// `leads` edge and inherited through `in-epic`) also sees the status.
//
// Both blocks matter. A type no role gates with `visible:` needs no field
// verdict, so a redaction never resolves local roles and a per-row lookup
// passes the budget unseen. readsFor's realistic pass has no such block.
func fieldPolicy() *acl.Policy {
	p := realisticBudgetPolicy()
	editor := p.Roles["editor"]
	editor.Visible = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}
	p.Roles["editor"] = editor
	lead := p.Roles["lead"]
	lead.Read = []string{"ticket"}
	lead.Visible = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}, {Field: "status"}}}
	p.Roles["lead"] = lead
	return p
}

// useFieldPolicy makes P1 lead EP1, installs [fieldPolicy] with the
// policy-backed field resolver and the query-scope resolver that binds
// current_user, as cmd/rela-server wires them, and resets the counter.
func useFieldPolicy(t *testing.T, app *App, counting *storetest.Counting) *acl.Declarative {
	t.Helper()
	_, err := counting.CreateRelation(context.Background(),
		entity.RelationKey{From: "P1", Type: "leads", To: "EP1"}, nil)
	require.NoError(t, err)
	d := mustNewACL(t, fieldPolicy(), app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", app.Meta(), app.store, d)
	require.NoError(t, err)
	setTestResolver(t, app, res)
	require.NoError(t, app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)))
	wireRealViewConditions(t, app)
	counting.Reset()
	return d
}

// fieldPolicyReads runs op at 10 and 50 tickets under [useFieldPolicy] and
// returns the store reads of each run. Setup reads are not counted.
func fieldPolicyReads(t *testing.T, install func(*App), op func(*App, context.Context)) (small, large int, detail string) {
	t.Helper()
	counts := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		app, counting, _, ctx := newBudgetAppOn(t, n, memstore.New())
		install(app)
		d := useFieldPolicy(t, app, counting)
		op(app, gateCtxFor(ctx, t, d))
		counts = append(counts, counting.Reads())
		detail = counting.String()
	}
	return counts[0], counts[1], detail
}

// leadDoneCount is how many of the fixture's done tickets lie in EP1, the
// only tickets whose status P1 may see under [fieldPolicy]. The fixture puts
// tickets 1 to budgetTicketsPerEpic in EP1 and marks odd-numbered tickets
// done.
func leadDoneCount() int {
	n := 0
	for i := 1; i <= budgetTicketsPerEpic; i++ {
		if i%2 == 1 {
			n++
		}
	}
	return n
}

// installDoneStatus gives one nav entry over list a rule counting done
// tickets, which reads the status that only `lead` may see.
func installDoneStatus(app *App, list dataentryconfig.List) {
	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"tickets": list},
		[]dataentryconfig.NavigationEntry{{Label: "Tickets", List: "tickets", Status: []dataentryconfig.NavStatusRule{
			rule("error", "{count} done", "entity.status == 'done'"),
		}}})
}

// The status markers redact every row of the entry's list before the list
// condition and each rule judge it. Unprimed, each redaction looked up the
// principal's local roles per row: 867 statements per request on Atlas
// (TKT-YLG695).
func TestQueryBudget_NavStatusIsSizeIndependent(t *testing.T) {
	small, large, detail := fieldPolicyReads(t,
		func(app *App) {
			installDoneStatus(app, dataentryconfig.List{
				EntityType: "ticket",
				Condition:  "related(entity, 'assigned-to', { id = current_user.id })",
			})
		},
		func(app *App, ctx context.Context) {
			if got := navStatusAs(ctx, t, app); got["0"].Count == 0 {
				t.Fatalf("no marker counted, so this budget proves nothing: %v", got)
			}
		})
	assertBudget(t, "nav status", small, large, navStatusFieldPolicyBudget, detail)
}

// Without a list condition, only the rules redact, and they are primed too.
func TestQueryBudget_NavStatusRuleOnlyIsSizeIndependent(t *testing.T) {
	small, large, detail := fieldPolicyReads(t,
		func(app *App) { installDoneStatus(app, dataentryconfig.List{EntityType: "ticket"}) },
		func(app *App, ctx context.Context) {
			if got := navStatusAs(ctx, t, app); got["0"].Count == 0 {
				t.Fatalf("no marker counted, so this budget proves nothing: %v", got)
			}
		})
	assertBudget(t, "nav status (rule only)", small, large, navStatusRuleOnlyFieldPolicyBudget, detail)
}

// A generated sidebar group redacts each of its rows before taking the title.
func TestQueryBudget_NavItemsRedactionIsSizeIndependent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		list   dataentryconfig.List
		pinned int
	}{
		{"nav items", dataentryconfig.List{EntityType: "ticket"}, navItemsFieldPolicyBudget},
		{"nav items (condition)", dataentryconfig.List{
			EntityType: "ticket", Condition: "related(entity, 'assigned-to', { id = current_user.id })",
		}, navItemsConditionFieldPolicyBudget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			small, large, detail := fieldPolicyReads(t,
				func(app *App) {
					installNavItemsGroup(app, tc.list, dataentryconfig.NavItemsFrom{Limit: 100})
				},
				func(app *App, ctx context.Context) {
					req := httptest.NewRequest(http.MethodGet, "/api/v1/_nav_items", http.NoBody).WithContext(ctx)
					rec := httptest.NewRecorder()
					handleV1NavItems(app, rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("nav items: %d %s", rec.Code, rec.Body)
					}
				})
			assertBudget(t, tc.name, small, large, tc.pinned, detail)
		})
	}
}

// Under the prime, a status visible only through a local role inherited from
// the epic still counts, and one hidden from every role held on the row does
// not.
func TestNavStatus_PrimedCountHonorsLocalRoleVisibility(t *testing.T) {
	app, counting, _, ctx := newBudgetAppOn(t, 20, memstore.New())
	installDoneStatus(app, dataentryconfig.List{EntityType: "ticket"})
	d := useFieldPolicy(t, app, counting)

	got := navStatusAs(gateCtxFor(ctx, t, d), t, app)
	require.Equal(t, leadDoneCount(), got["0"].Count, "only EP1's done tickets show their status to P1")
	require.NotContains(t, counting.String(), "GetRelation", "the count resolved roles per row")
}
