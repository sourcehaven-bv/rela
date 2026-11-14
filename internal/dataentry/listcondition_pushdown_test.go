package dataentry

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// conditionLists are the list `condition:`s the comparison serves, over the
// fixture of [newScopePushdownAppOn].
var conditionLists = map[string]string{
	"open":       "entity.status == 'open'",
	"not-done":   "entity.status ~= 'done' and entity.status ~= 'blocked'",
	"impl-open":  "related(entity, 'implements', { status = 'open' })",
	"unreported": "not related(entity, 'reportedBy')",
	// Denied for dave, who reads no users: an empty page in the store.
	"reported": "related(entity, 'reportedBy')",
	// The sign-aware dedup must keep both terms: no row matches.
	"contradiction":    "related(entity, 'reportedBy') and not related(entity, 'reportedBy')",
	"mine":             "is_current_user(entity.assignee)",
	"watching":         "has_current_user(entity.watchers)",
	"unassigned-open":  "entity.assignee ~= current_user.id and entity.status == 'open'",
	"open-or-reported": "entity.status == 'open' or related(entity, 'reportedBy')",
}

// statusConditions are the conditions that read status, which frank may not
// see on every row; they must stay on the Go path for him.
var statusConditions = map[string]bool{
	"open": true, "not-done": true, "unassigned-open": true, "open-or-reported": true,
}

// deniedForDave are the conditions with a positive traversal into users,
// which dave may not read.
var deniedForDave = map[string]bool{"reported": true, "contradiction": true}

// withConditionLists configures [conditionLists] on app and installs the
// fixture's ACL plus frank, whose role shows every ticket field but status.
// It wires the policy-backed field resolver, so a condition is judged on the
// redacted row as in production.
func withConditionLists(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	for id, cond := range conditionLists {
		app.Cfg().Lists[id] = dataentryconfig.List{EntityType: "ticket", Condition: cond}
	}
	wireRealViewConditions(t, app)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all":      {Read: []string{"ticket", "feature", "user"}},
			"browse":   {Read: []string{"feature", "user"}},
			"tickets":  {Read: []string{"ticket", "feature"}},
			"features": {Read: []string{"feature"}},
			"editor":   {Read: []string{"ticket"}},
			"nostatus": {
				Read: []string{"ticket", "feature", "user"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"}, {Field: "assignee"}, {Field: "watchers"},
				}},
			},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		Assignments: map[string]string{
			"alice": "all", "bob": "browse", "dave": "tickets", "erin": "features", "frank": "nostatus",
		},
	}, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", app.Meta(), app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, app, res)
	return d
}

func TestListPushdown_ConditionMatchesGoPath(t *testing.T) {
	app, _, counting := newScopePushdownAppOn(t, memstore.New())
	assertConditionMatchesGoPath(t, app, counting)
}

// assertConditionMatchesGoPath serves every list condition for every
// principal through the list API, and compares each page and total with the
// Go path: the scoped rows, judged by the condition on the redacted row
// (TKT-I39Y7N4). It also checks which path served the page, so the
// comparison cannot pass by both sides taking the Go path.
func assertConditionMatchesGoPath(t *testing.T, app *App, counting *storetest.Counting) {
	t.Helper()
	d := withConditionLists(t, app)
	for _, user := range []string{"alice", "bob", "dave", "erin", "frank"} {
		for list := range conditionLists {
			for _, rq := range []string{"", "sort=title", "sort=-title", "filter%5Btitle%5D=a&sort=title"} {
				for _, pg := range []struct{ page, perPage int }{{1, 3}, {2, 3}, {1, 50}} {
					q := "list_id=" + list
					if rq != "" {
						q += "&" + rq
					}
					q += fmt.Sprintf("&per_page=%d&page=%d", pg.perPage, pg.page)
					t.Run(user+" "+q, func(t *testing.T) {
						counting.Reset()
						resp, rec := listEntitiesAs(principalCtx(user), t, app, d, "ticket", "tickets", q)
						if rec.Code != http.StatusOK {
							t.Fatalf("status %d: %s", rec.Code, rec.Body)
						}
						calls := counting.Calls()

						wantIDs, wantTotal := conditionGoPath(t, app, d, user, list, rq, pg.page, pg.perPage)
						gotIDs := make([]string, 0, len(resp.Data))
						for _, e := range resp.Data {
							gotIDs = append(gotIDs, e.ID)
						}
						if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
							t.Errorf("rows differ: served %v, go %v", gotIDs, wantIDs)
						}
						if resp.Meta.Total != wantTotal {
							t.Errorf("total: served %d, go %d", resp.Meta.Total, wantTotal)
						}

						// erin reads no tickets, so neither path reaches a
						// row. Otherwise CountMatched is the pushed path's
						// signature: the Go path counts its slice in memory.
						if user == "erin" {
							return
						}
						pushed := list != "open-or-reported" && (user != "frank" || !statusConditions[list])
						if user == "dave" && deniedForDave[list] {
							// A denied positive traversal is an empty page:
							// nothing to count, and no row is read.
							if calls["CountMatched"] != 0 || calls["MatchingFaces"] != 0 || calls["GraphQueryHeaders"] != 0 {
								t.Errorf("%s should be an empty page without a read for dave: %s", list, counting)
							}
							return
						}
						if pushed && (calls["CountMatched"] != 1 || calls["MatchingFaces"] != 0) {
							t.Errorf("%s was not pushed down for %s: %s", list, user, counting)
						}
						if !pushed && calls["CountMatched"] != 0 {
							t.Errorf("%s must stay on the Go path for %s: %s", list, user, counting)
						}
					})
				}
			}
		}
	}
}

// conditionGoPath is the Go path for list under rq: every row the list shows
// user before its condition, then the condition judged on each redacted row,
// then the page.
func conditionGoPath(
	t *testing.T, app *App, d *acl.Declarative, user, list, rq string, page, perPage int,
) (ids []string, total int) {
	t.Helper()
	ctx, err := bindQueryIdentity(gateCtxFor(principalCtx(user), t, d), mustScope(t, app, ""))
	if err != nil {
		t.Fatal(err)
	}
	all, err := app.scopedSortedEntities(ctx, "ticket", parseQuery(rq))
	if err != nil {
		t.Fatal(err)
	}
	ctx = primeVerdicts(ctx, app.fieldResolver, all)
	m := viewCondition(app.viewConditions, app.State(), viewKindList, list)
	if m == nil {
		t.Fatalf("list %q has no compiled condition", list)
	}
	kept, err := applyViewCondition(ctx, all, m, app.redactedForSuggestion, app.Services().Store)
	if err != nil {
		t.Fatal(err)
	}
	start := min((page-1)*perPage, len(kept))
	end := min(start+perPage, len(kept))
	for _, e := range kept[start:end] {
		ids = append(ids, e.ID)
	}
	return ids, len(kept)
}
