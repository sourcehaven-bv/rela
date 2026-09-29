package dataentry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// installNavStatusConfig publishes a copy of the app's config with the given
// lists added and the navigation replaced, as installDashboardConfig does.
func installNavStatusConfig(app *App, lists map[string]dataentryconfig.List, nav []dataentryconfig.NavigationEntry) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = make(map[string]dataentryconfig.List, len(cur.Cfg.Lists)+len(lists))
	maps.Copy(cfg.Lists, cur.Cfg.Lists)
	maps.Copy(cfg.Lists, lists)
	cfg.Navigation = nav
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// navStatusAs calls the endpoint with ctx and decodes its items.
func navStatusAs(ctx context.Context, t *testing.T, app *App) map[string]v1.NavStatus {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_nav_status", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1NavStatus(app, rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store",
		"the counts are per principal, so no shared cache may hold them")
	var resp v1.NavStatusResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	return resp.Items
}

func rule(tone, label, condition string) dataentryconfig.NavStatusRule {
	return dataentryconfig.NavStatusRule{Tone: tone, Label: label, Condition: condition}
}

// The first rule with a count above zero is the entry's status; an entry
// where every rule counts zero is absent rather than served as a zero.
func TestNavStatus_FirstMatchWinsAndZeroIsOmitted(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	wireRealViewConditions(t, app)
	seedAssignedTicket(app, "TKT-1", "alice")
	seedAssignedTicket(app, "TKT-2", "bob")
	seedEntity(app, &entity.Entity{ID: "TKT-3", Type: "ticket",
		Properties: map[string]any{"title": "Done", "status": "done"}})

	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{
			{Label: "Home", Dashboard: true},
			{Group: "Work", Items: []dataentryconfig.NavigationEntry{
				{Label: "All", List: "all", Status: []dataentryconfig.NavStatusRule{
					rule("error", "{count} blocked", "entity.status == 'blocked'"),
					rule("warning", "{count} open", "entity.status == 'open'"),
					rule("new", "{count} in total", ""),
				}},
				{Label: "Blocked", List: "all", Status: []dataentryconfig.NavStatusRule{
					rule("error", "{count} blocked", "entity.status == 'blocked'"),
				}},
				{Label: "Everything", List: "all", Status: []dataentryconfig.NavStatusRule{
					rule("info", "{count} tickets", ""),
				}},
			}},
		})

	got := navStatusAs(principalCtx("alice"), t, app)
	require.Equal(t, map[string]v1.NavStatus{
		"1.0": {Tone: "warning", Label: "2 open", Count: 2},
		"1.2": {Tone: "info", Label: "3 tickets", Count: 3},
	}, got)
}

// A rule condition naming current_user counts for the principal asking.
func TestNavStatus_CurrentUserCondition(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	wireRealViewConditions(t, app)
	// The resolver binds current_user, as cmd/rela-server wires it.
	require.NoError(t, app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)))
	seedAssignedTicket(app, "TKT-1", "alice")
	seedAssignedTicket(app, "TKT-2", "alice")
	seedAssignedTicket(app, "TKT-3", "bob")

	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{
			{Label: "All", List: "all", Status: []dataentryconfig.NavStatusRule{
				rule("new", "{count} mine", "is_current_user(entity.assignee)"),
			}},
		})

	require.Equal(t, 2, navStatusAs(principalCtx("alice"), t, app)["0"].Count)
	require.Equal(t, 1, navStatusAs(principalCtx("bob"), t, app)["0"].Count)
	require.Empty(t, navStatusAs(principalCtx("carol"), t, app))
}

// The count is taken over exactly the rows the list shows: its query scope,
// its static filters and its condition all narrow it. Each narrowing alone
// admits a different set, so a count that dropped any of them would differ.
func TestNavStatus_CountsTheListsOwnRows(t *testing.T) {
	// TAAK-1 todo/alice, TAAK-2 gereed/bob, TAAK-3 gearchiveerd/alice; the
	// type's default scope hides the archived row.
	app := newScopeTestApp(t)
	wireRealViewConditions(t, app)

	count := func(list dataentryconfig.List) int {
		t.Helper()
		installNavStatusConfig(app,
			map[string]dataentryconfig.List{"taken": list},
			[]dataentryconfig.NavigationEntry{
				{Label: "Taken", List: "taken", Status: []dataentryconfig.NavStatusRule{rule("new", "{count}", "")}},
			})
		return navStatusAs(principalCtx("alice"), t, app)["0"].Count
	}

	notArchived := []dataentryconfig.FilterConfig{{Property: "status", Operator: "!=", Value: "gearchiveerd"}}
	require.Equal(t, 2, count(dataentryconfig.List{EntityType: "taak"}), "the type's default scope applies")
	require.Equal(t, 3, count(dataentryconfig.List{EntityType: "taak", QueryScope: "all"}))
	require.Equal(t, 2, count(dataentryconfig.List{EntityType: "taak", QueryScope: "all", Filters: notArchived}))
	require.Equal(t, 1, count(dataentryconfig.List{
		EntityType: "taak", QueryScope: "all", Filters: notArchived,
		Condition: "entity.toegewezen_aan == 'alice'",
	}), "scope, static filter and list condition must all apply")
}

// Rows the principal may not read are not counted: the count is taken after
// the ACL row gate, never over the raw store.
func TestNavStatus_CountsOnlyReadableRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Visible"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-002", "belongs-to", "PRJ-9"))

	d := mustNewACL(t, &acl.Policy{
		Roles:               map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d

	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{
			{Label: "All", List: "all", Status: []dataentryconfig.NavStatusRule{rule("new", "{count}", "")}},
		})

	got := navStatusAs(gateCtxFor(aliceCtx(), t, d), t, app)
	require.Equal(t, 1, got["0"].Count, "only the ticket alice may read is counted")
	require.Equal(t, "1", got["0"].Label)
}

// An entry the principal does not see in the sidebar gets no status either.
func TestNavStatus_PermissionGatedEntryIsOmitted(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T"}})
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{
			{Label: "Audit", List: "all", Permission: "admin:read",
				Status: []dataentryconfig.NavStatusRule{rule("new", "{count}", "")}},
		})

	require.Contains(t, navStatusAs(gateCtxFor(aliceCtx(), t, d), t, app), "0", "the holder sees the status")
	require.Empty(t, navStatusAs(gateCtxFor(principalCtx("bob"), t, d), t, app),
		"a non-holder does not see the entry, so gets no status for it")
}

// A rule whose condition is declared but not compiled is skipped, never
// counted as if it had no condition.
func TestNavStatus_UncompiledConditionSkipsTheEntry(t *testing.T) {
	app := newTestAppV1(t) // view conditions deliberately not wired
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T"}})

	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{
			{Label: "All", List: "all", Status: []dataentryconfig.NavStatusRule{
				rule("error", "{count}", "entity.status == 'blocked'"),
			}},
			{Label: "Count", List: "all", Status: []dataentryconfig.NavStatusRule{rule("new", "{count}", "")}},
		})

	got := navStatusAs(principalCtx("alice"), t, app)
	require.NotContains(t, got, "0")
	require.Equal(t, 1, got["1"].Count, "an entry that fails does not blank the others")
}

// The sidebar names each entry that declares status rules by its key, and
// carries no counts.
func TestNavStatus_SidebarCarriesOnlyTheKey(t *testing.T) {
	app := newTestAppV1(t)
	status := []dataentryconfig.NavStatusRule{rule("new", "{count}", "")}
	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{
			{Label: "Top", List: "all", Status: status},
			{Group: "Work", Items: []dataentryconfig.NavigationEntry{
				{Label: "Plain", List: "all"},
				{Label: "Mine", List: "all", Status: status},
			}},
		})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp v1.SidebarResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	keys := map[string]string{}
	for _, g := range resp.Navigation {
		for _, it := range g.Items {
			keys[it.Label] = it.StatusKey
		}
	}
	require.Equal(t, map[string]string{"Top": "0", "Plain": "", "Mine": "1.1"}, keys)
}

func TestNavStatusListQuery_MirrorsTheSPA(t *testing.T) {
	got := navStatusListQuery(dataentryconfig.List{Filters: []dataentryconfig.FilterConfig{
		{Property: "status", Operator: "!=", Value: "done"},
		{Property: "status", Operator: "!=", Value: "archived"},
		{Property: "prio", Operator: ">=", Value: "2"},
		{Property: "skipped", Operator: "=", Value: ""},
	}})
	require.Equal(t, map[string][]string{
		"filter[status][ne]": {"done,archived"},
		"filter[prio][gte]":  {"2"},
	}, got)
}
