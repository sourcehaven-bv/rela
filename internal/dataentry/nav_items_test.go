package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// navItemsAs calls the endpoint with ctx and returns the decoded items and
// the raw body, for leak checks.
func navItemsAs(ctx context.Context, t *testing.T, app *App) (items map[string]v1.NavItemList, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_nav_items", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1NavItems(app, rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store",
		"the entries are per principal, so no shared cache may hold them")
	body = rec.Body.String()
	var resp v1.NavItemsResponse
	require.NoError(t, json.Unmarshal([]byte(body), &resp))
	return resp.Items, body
}

// installNavItemsGroup publishes a navigation of one generated group over a
// list of tickets. The group's key is "0".
func installNavItemsGroup(app *App, list dataentryconfig.List, from dataentryconfig.NavItemsFrom) {
	from.List = "tickets"
	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"tickets": list},
		[]dataentryconfig.NavigationEntry{{Group: "Tickets", ItemsFrom: &from}})
}

// entryIDs returns the ids of a group's entries, in order.
func entryIDs(list v1.NavItemList) []string {
	ids := make([]string, 0, len(list.Entries))
	for _, e := range list.Entries {
		ids = append(ids, e.ID)
	}
	return ids
}

// seedGatedTickets seeds three tickets of which alice may read two, through
// a project she edits, and installs that policy on the app.
func seedGatedTickets(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Alpha"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "SECRET-ROW"}})
	seedEntity(app, &entity.Entity{ID: "TKT-003", Type: "ticket", Properties: map[string]any{"title": "Gamma"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-002", "belongs-to", "PRJ-9"))
	seedRelation(app, entity.NewRelation("TKT-003", "belongs-to", "PRJ-42"))

	d := mustNewACL(t, &acl.Policy{
		Roles:               map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d
	return d
}

// The entries are the rows the list shows the principal, in the list's
// order, labeled with their titles. A row the principal may not read is
// absent, id and title both.
func TestNavItems_RowsAreTheListsReadableRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedGatedTickets(t, app)
	installNavItemsGroup(app,
		dataentryconfig.List{EntityType: "ticket", Sort: []dataentryconfig.SortSpec{{Property: "title", Direction: "desc"}}},
		dataentryconfig.NavItemsFrom{})

	items, body := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	require.Equal(t, []v1.NavItem{
		{ID: "TKT-003", Type: "ticket", Label: "Gamma"},
		{ID: "TKT-001", Type: "ticket", Label: "Alpha"},
	}, items["0"].Entries, "readable rows, in the list's sort order")
	require.False(t, items["0"].Truncated)
	require.NotContains(t, body, "TKT-002")
	require.NotContains(t, body, "SECRET-ROW")
}

// The list's own narrowing applies: its static filters and its condition.
func TestNavItems_ListFiltersAndConditionApply(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	for _, tk := range []struct{ id, status string }{{"TKT-1", "open"}, {"TKT-2", "done"}, {"TKT-3", "open"}} {
		seedEntity(app, &entity.Entity{ID: tk.id, Type: "ticket",
			Properties: map[string]any{"title": "T " + tk.id, "status": tk.status}})
	}
	installNavItemsGroup(app, dataentryconfig.List{
		EntityType: "ticket",
		Filters:    []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}},
		Condition:  "entity.title ~= 'T TKT-3'",
	}, dataentryconfig.NavItemsFrom{})

	items, _ := navItemsAs(principalCtx("alice"), t, app)
	require.Equal(t, []string{"TKT-1"}, entryIDs(items["0"]))
}

// Truncated says the list holds more rows than the limit, counted AFTER the
// row gate: three tickets exist and two are readable, so a limit of two is
// not truncated for alice, while it is for a principal who reads all three.
func TestNavItems_TruncatedIsPostFilter(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedGatedTickets(t, app)
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{Limit: 2})

	items, _ := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	require.Len(t, items["0"].Entries, 2)
	require.False(t, items["0"].Truncated, "a hidden row must not show up as a truncation")

	app.acl = acl.NopACL{}
	items, _ = navItemsAs(principalCtx("alice"), t, app)
	require.Len(t, items["0"].Entries, 2)
	require.True(t, items["0"].Truncated)
}

// A hidden display property never reaches the label, and a property initial
// over a hidden property gives no badge.
func TestNavItems_RedactedTitleDoesNotLeak(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "SECRET-TITLE", "status": "open"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{
		Initial: &dataentryconfig.NavItemsInitial{Property: "title"},
	})

	items, body := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	require.Equal(t, []v1.NavItem{{ID: "TKT-001", Type: "ticket", Label: "TKT-001"}}, items["0"].Entries)
	require.NotContains(t, body, "SECRET")
}

// A property initial is the first letter of the row's own property.
func TestNavItems_PropertyInitial(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "T", "status": " open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "U"}})
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{
		Initial: &dataentryconfig.NavItemsInitial{Property: "status"},
	})

	items, _ := navItemsAs(principalCtx("alice"), t, app)
	require.Equal(t, "O", items["0"].Entries[0].Initial)
	require.Empty(t, items["0"].Entries[1].Initial, "an unset property gives no badge")
}

// A relation initial is the first letter of a related entity's title, the
// lowest id winning when there are several. A related entity the principal
// may not read gives none, and its title never reaches the response.
func TestNavItems_RelationInitial(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "One"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "Two"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "zulu owner"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "HIDDEN-OWNER"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-2"))
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-1"))
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"full":    {Read: []string{"ticket", "feature"}},
			"tickets": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "full", "bob": "tickets"},
	}, app.store)
	app.acl = d
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{
		Initial: &dataentryconfig.NavItemsInitial{Relation: "implements"},
	})

	items, _ := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	require.Equal(t, "H", items["0"].Entries[0].Initial, "FEAT-1 has the lowest id")
	require.Empty(t, items["0"].Entries[1].Initial, "a row with no related entity has no badge")

	items, body := navItemsAs(gateCtxFor(principalCtx("bob"), t, d), t, app)
	require.Len(t, items["0"].Entries, 2)
	require.Empty(t, items["0"].Entries[0].Initial, "a hidden owner gives no badge")
	require.NotContains(t, body, "HIDDEN-OWNER")
	require.NotContains(t, body, "FEAT-")
}

// A related entity whose display property is hidden gives no badge rather
// than a letter of its id or of the hidden value.
func TestNavItems_RelationInitialRedactsTheNeighbour(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "One"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "Quiet"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-1"))
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{
		Initial: &dataentryconfig.NavItemsInitial{Relation: "implements"},
	})

	items, _ := navItemsAs(principalCtx("alice"), t, app)
	require.Empty(t, items["0"].Entries[0].Initial)
}

// The sidebar carries the group's key and config names but no entry, so it
// is byte-identical for principals who see different rows.
func TestNavItems_SidebarIdenticalAcrossPrincipals(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedGatedTickets(t, app)
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{})

	sidebar := func(ctx context.Context) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(ctx)
		rec := httptest.NewRecorder()
		app.views.handleV1Sidebar(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return rec.Body.String()
	}
	asAlice := sidebar(gateCtxFor(aliceCtx(), t, d))
	require.Equal(t, asAlice, sidebar(gateCtxFor(principalCtx("bob"), t, d)))

	var resp v1.SidebarResponse
	require.NoError(t, json.Unmarshal([]byte(asAlice), &resp))
	require.Len(t, resp.Navigation, 1, "a generated group stays although it has no static items")
	require.Equal(t, "0", resp.Navigation[0].ItemsKey)
	require.Equal(t, "tickets", resp.Navigation[0].ItemsList)
	require.Empty(t, resp.Navigation[0].Items)
	require.NotContains(t, asAlice, "TKT-")
	require.NotContains(t, asAlice, "Alpha")
}

// A generated group with `create: true` carries a create offer for the list's
// type, to a principal who may create it, when a form resolves for it.
func TestNavItems_CreateOffer(t *testing.T) {
	app := newTestAppV1(t)
	installNavItemsGroup(app, dataentryconfig.List{EntityType: "ticket"}, dataentryconfig.NavItemsFrom{Create: true})
	cfg := *app.State().Cfg
	cfg.Forms = map[string]dataentryconfig.Form{"new_ticket": {EntityType: "ticket"}}
	next := *app.State()
	next.Cfg = &cfg
	app.schema.Publish(&next)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"author": {Read: []string{"ticket"}, Create: []string{"ticket"}},
			"reader": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "author", "carol": "reader"},
	}, app.store)
	app.acl = d

	cases := []struct {
		user string
		want *v1.SidebarCreate
	}{
		{"alice", &v1.SidebarCreate{Type: "ticket", Label: "Ticket", Form: "new_ticket"}},
		{"carol", nil},
	}
	for _, tc := range cases {
		t.Run(tc.user, func(t *testing.T) {
			resp, _ := sidebarFor(gateCtxFor(principalCtx(tc.user), t, d), t, app, "")
			require.Len(t, resp.Navigation, 1)
			require.Equal(t, tc.want, resp.Navigation[0].ItemsCreate)
		})
	}
}
