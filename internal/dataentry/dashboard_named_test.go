package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// Named dashboards (TKT-5LQNLY): `dashboards:` next to the default
// `dashboard:`, reached by `dashboard: <name>` and `/_dashboard?name=`.

// installNamedDashboards publishes a default dashboard and a named "crm"
// dashboard whose middle card is gated on admin:read.
func installNamedDashboards(app *App) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboard = &dataentryconfig.DashboardConfig{
		Title: "Overview",
		Cards: []dataentryconfig.DashboardCard{{Title: "Everything", Query: "type:ticket", Display: "count"}},
	}
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{
		"crm": {
			Title:       "Pipeline",
			Description: "Deals",
			Cards: []dataentryconfig.DashboardCard{
				{Title: "Open deals", Query: "type:ticket", Display: "count"},
				{Title: "Margins", Query: "type:ticket", Display: "count", Permission: "admin:read"},
				{Title: "Won", Query: "type:ticket", Display: "count"},
			},
		},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// namedDashboard performs GET /_dashboard with the given raw query string.
func namedDashboard(ctx context.Context, t *testing.T, app *App, query string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/v1/_dashboard"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Dashboard(rec, req)
	return rec
}

func decodeDashboard(t *testing.T, rec *httptest.ResponseRecorder) v1.DashboardResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp v1.DashboardResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func cardTitles(resp v1.DashboardResponse) []string {
	out := make([]string, 0, len(resp.Cards))
	for _, c := range resp.Cards {
		out = append(out, c.Title)
	}
	return out
}

func TestNamedDashboard_ServesByName(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)

	resp := decodeDashboard(t, namedDashboard(t.Context(), t, app, "name=crm"))
	require.Equal(t, "Pipeline", resp.Title)
	require.Equal(t, "Deals", resp.Description)
	require.Equal(t, []string{"Open deals", "Margins", "Won"}, cardTitles(resp))
}

func TestNamedDashboard_DefaultWithoutName(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)

	for _, query := range []string{"", "name=", "name=default"} {
		t.Run(query, func(t *testing.T) {
			resp := decodeDashboard(t, namedDashboard(t.Context(), t, app, query))
			require.Equal(t, "Overview", resp.Title)
			require.Equal(t, []string{"Everything"}, cardTitles(resp))
		})
	}
}

func TestNamedDashboard_UnknownNameIs404(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)

	rec := namedDashboard(t.Context(), t, app, "name="+url.QueryEscape("nope"))
	require.Equal(t, http.StatusNotFound, rec.Code)
	var problem v1.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	require.Equal(t, "https://rela.dev/errors/dashboard_not_found", problem.Type)
	require.Contains(t, problem.Detail, `"nope"`)
}

// A named dashboard filters its cards exactly as the default one does.
func TestNamedDashboard_PermissionFiltering(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	setTestACL(t, app, d)

	holder := decodeDashboard(t, namedDashboard(gateCtxFor(aliceCtx(), t, d), t, app, "name=crm"))
	require.Equal(t, []string{"Open deals", "Margins", "Won"}, cardTitles(holder))

	other := decodeDashboard(t, namedDashboard(gateCtxFor(principalCtx("bob"), t, d), t, app, "name=crm"))
	require.Equal(t, []string{"Open deals", "Won"}, cardTitles(other))
}

// A named dashboard with no cards serializes cards as [], like the default.
func TestNamedDashboard_EmptyCardsIsArray(t *testing.T) {
	app := newTestAppV1(t)
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{"empty": {Title: "Empty"}}
	next.Cfg = &cfg
	app.schema.Publish(&next)

	rec := namedDashboard(t.Context(), t, app, "name=empty")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"title":"Empty","cards":[]}`, rec.Body.String())
}

// The sidebar links a named dashboard at /dashboard/<name>; a page tab and a
// space home name it too.
func TestNamedDashboard_SidebarPageAndSpaceHome(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)
	crm := dataentryconfig.DashboardRef("crm")
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		map[string]dataentryconfig.Page{"sales": {Label: "Sales", Tabs: []dataentryconfig.PageTab{
			{ID: "pipeline", Label: "Pipeline", Dashboard: crm},
			{ID: "home", Label: "Home", Dashboard: dataentryconfig.DefaultDashboard},
		}}},
		[]dataentryconfig.NavigationEntry{
			{Label: "Home", Dashboard: dataentryconfig.DefaultDashboard},
			{Label: "Pipeline", Dashboard: crm},
			{Page: "sales"},
		})

	resp, _ := sidebarFor(t.Context(), t, app, "")
	require.Equal(t, []v1.SidebarItem{
		{Label: "Home", Href: "/", Icon: "dashboard"},
		{Label: "Pipeline", Href: "/dashboard/crm", Icon: "dashboard"},
		{Label: "Sales", Href: "/p/sales", Icon: "dashboard"},
	}, resp.Navigation[0].Items)
	require.Equal(t, []v1.SidebarPageTab{
		{ID: "pipeline", Label: "Pipeline", View: "dashboard", Target: "crm"},
		{ID: "home", Label: "Home", View: "dashboard"},
	}, resp.Pages["sales"].Tabs)

	installSpacesConfig(app, nil, nil, []dataentryconfig.Space{
		{ID: "crm", Label: "CRM", Home: &dataentryconfig.NavigationEntry{Dashboard: crm}},
		{ID: "main", Label: "Main", Home: &dataentryconfig.NavigationEntry{Dashboard: dataentryconfig.DefaultDashboard}},
	})
	resp, _ = sidebarFor(t.Context(), t, app, "crm")
	homes := map[string]string{}
	for _, sp := range resp.Spaces {
		homes[sp.ID] = sp.Home
	}
	require.Equal(t, map[string]string{"crm": "/dashboard/crm", "main": "/dashboard"}, homes)
}

// `_config` serves `dashboards:` verbatim and a nav entry's ref as true or
// the name, so a client that knows only the boolean keeps working.
func TestNamedDashboard_ConfigWireShape(t *testing.T) {
	app := newTestAppV1(t)
	installNamedDashboards(app)
	installPagesConfig(app, nil, nil, []dataentryconfig.NavigationEntry{
		{Label: "Home", Dashboard: dataentryconfig.DefaultDashboard},
		{Label: "Pipeline", Dashboard: "crm"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Config(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var raw struct {
		Dashboards map[string]json.RawMessage `json:"dashboards"`
		Navigation []map[string]any           `json:"navigation"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Contains(t, raw.Dashboards, "crm")
	require.Equal(t, true, raw.Navigation[0]["dashboard"])
	require.Equal(t, "crm", raw.Navigation[1]["dashboard"])
}

// A command scoped to a dashboard shows on that dashboard only: `true` on the
// default one, a name on the named one. An unscoped global command shows on
// every dashboard.
func TestNamedDashboard_CommandScope(t *testing.T) {
	app, _ := testAppInstance()
	app.Cfg().Commands = map[string]CommandConfig{
		"on-default": {Label: "D", Script: "echo", Context: "global",
			AvailableOn: &CommandScope{Dashboard: dataentryconfig.DefaultDashboard}},
		"on-crm": {Label: "C", Script: "echo", Context: "global",
			AvailableOn: &CommandScope{Dashboard: "crm"}},
		"anywhere": {Label: "A", Script: "echo", Context: "global"},
	}

	onDefault := cmdIDs(app.commands.resolveCommands(t.Context(), "dashboard", "", ""))
	require.ElementsMatch(t, []string{"on-default", "anywhere"}, onDefault)

	asDefault := cmdIDs(app.commands.resolveCommands(t.Context(), "dashboard", "default", ""))
	require.ElementsMatch(t, []string{"on-default", "anywhere"}, asDefault, "qualifier default names the default dashboard")

	onCRM := cmdIDs(app.commands.resolveCommands(t.Context(), "dashboard", "crm", ""))
	require.ElementsMatch(t, []string{"on-crm", "anywhere"}, onCRM)
}

// The access log keeps a named dashboard's name in /dashboard/<name>, as it
// does for list and page names.
func TestNamedDashboard_RouteWords(t *testing.T) {
	words := buildRouteWords(&Schema{Cfg: &dataentryconfig.Config{
		Dashboards: map[string]dataentryconfig.DashboardConfig{"crm": {Title: "Pipeline"}},
	}})
	require.True(t, words["crm"])
}
