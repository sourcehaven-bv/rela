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
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Spaces (TKT-GNKR5H): with `spaces:` configured the sidebar serves the
// switcher, the current space's navigation and its Create menu. The space
// `permission:` is the same UX filter as a nav entry's.

// installSpacesConfig publishes a copy of the app's config with lists,
// forms and spaces set and the top-level navigation cleared.
func installSpacesConfig(
	app *App, lists map[string]dataentryconfig.List, forms map[string]dataentryconfig.Form,
	spaces []dataentryconfig.Space,
) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = make(map[string]dataentryconfig.List, len(cur.Cfg.Lists)+len(lists))
	maps.Copy(cfg.Lists, cur.Cfg.Lists)
	maps.Copy(cfg.Lists, lists)
	if forms != nil {
		cfg.Forms = forms
	}
	cfg.Navigation = nil
	cfg.Spaces = spaces
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// sidebarFor calls GET /api/v1/_sidebar with an optional ?space= and returns
// the decoded response and the raw body.
func sidebarFor(
	ctx context.Context, t *testing.T, app *App, space string,
) (resp v1.SidebarResponse, raw map[string]any) {
	t.Helper()
	target := "/api/v1/_sidebar"
	if space != "" {
		target += "?space=" + space
	}
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	return resp, raw
}

func spaceIDs(resp v1.SidebarResponse) []string {
	ids := make([]string, 0, len(resp.Spaces))
	for _, sp := range resp.Spaces {
		ids = append(ids, sp.ID)
	}
	return ids
}

func navLabels(resp v1.SidebarResponse) []string {
	var labels []string
	for _, g := range resp.Navigation {
		for _, it := range g.Items {
			labels = append(labels, it.Label)
		}
	}
	return labels
}

// Without `spaces:` the response carries none of the space fields.
func TestSpaces_NoSpacesLeavesSidebarUnchanged(t *testing.T) {
	app := newTestAppV1(t)
	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{{Label: "All", List: "all"}})

	resp, raw := sidebarFor(t.Context(), t, app, "anything")
	require.Equal(t, []string{"All"}, navLabels(resp))
	for _, key := range []string{"spaces", "space", "create"} {
		require.NotContains(t, raw, key)
	}
}

// resolutionSpaces is three spaces, the first gated on admin:read.
func resolutionSpaces() []dataentryconfig.Space {
	return []dataentryconfig.Space{
		{ID: "ops", Label: "Ops", Permission: "admin:read",
			Navigation: []dataentryconfig.NavigationEntry{{Label: "Ops list", List: "all"}}},
		{ID: "crm", Label: "CRM", Icon: "folder",
			Navigation: []dataentryconfig.NavigationEntry{{Label: "CRM list", List: "all"}}},
		{ID: "sales", Label: "Sales",
			Navigation: []dataentryconfig.NavigationEntry{{Label: "Sales list", List: "all"}}},
	}
}

func TestSpaces_Resolution(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil, resolutionSpaces())
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	cases := []struct {
		name       string
		user       string
		requested  string
		wantSpace  string
		wantSpaces []string
	}{
		{"no request picks the first", "alice", "", "ops", []string{"ops", "crm", "sales"}},
		{"requested space", "alice", "sales", "sales", []string{"ops", "crm", "sales"}},
		{"unknown id falls back to the first", "alice", "nope", "ops", []string{"ops", "crm", "sales"}},
		{"denied space falls back to the first permitted", "bob", "ops", "crm", []string{"crm", "sales"}},
		{"no request for a non-holder", "bob", "", "crm", []string{"crm", "sales"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := sidebarFor(gateCtxFor(principalCtx(tc.user), t, d), t, app, tc.requested)
			require.Equal(t, tc.wantSpace, resp.Space)
			require.Equal(t, tc.wantSpaces, spaceIDs(resp))
			require.Equal(t, []string{map[string]string{
				"ops": "Ops list", "crm": "CRM list", "sales": "Sales list",
			}[tc.wantSpace]}, navLabels(resp), "the navigation is the resolved space's")
		})
	}

	resp, _ := sidebarFor(gateCtxFor(principalCtx("bob"), t, d), t, app, "")
	require.Equal(t, v1.SidebarSpace{ID: "crm", Label: "CRM", Icon: "folder", Home: "/list/all"}, resp.Spaces[0])
}

// A hidden space's lists stay reachable: the space permission is a UX filter.
func TestSpaces_PermissionIsPresentationOnly(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil, resolutionSpaces())
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T"}})
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d
	ctx := gateCtxFor(principalCtx("bob"), t, d)

	resp, _ := sidebarFor(ctx, t, app, "ops")
	require.NotContains(t, spaceIDs(resp), "ops")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "TKT-1", "hiding a space must not gate the data behind it")
}

// With every space gated away there is no current space and no navigation.
func TestSpaces_NonePermitted(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil,
		resolutionSpaces()[:1])
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	resp, raw := sidebarFor(gateCtxFor(principalCtx("bob"), t, d), t, app, "ops")
	require.Empty(t, resp.Space)
	require.Empty(t, resp.Spaces)
	require.NotNil(t, resp.Navigation)
	require.Empty(t, resp.Navigation)
	require.NotContains(t, raw, "create")
}

// The Create menu offers a `create:` type only when a form resolves for it
// and the principal may create it, in config order.
func TestSpaces_CreateFiltering(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, nil,
		map[string]dataentryconfig.Form{
			"new_feature": {EntityType: "feature"},
			"new_ticket":  {EntityType: "ticket"},
		},
		[]dataentryconfig.Space{{ID: "crm", Label: "CRM", Create: []string{"feature", "ticket"}}})
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"author":   {Read: []string{"ticket", "feature"}, Create: []string{"ticket", "feature"}},
			"ticketer": {Read: []string{"ticket"}, Create: []string{"ticket"}},
			"reader":   {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "author", "bob": "ticketer", "carol": "reader"},
	}, app.store)
	app.acl = d

	cases := []struct {
		user string
		want []v1.SidebarCreate
	}{
		{"alice", []v1.SidebarCreate{
			{Type: "feature", Label: "Feature", Form: "new_feature"},
			{Type: "ticket", Label: "Ticket", Form: "new_ticket"},
		}},
		{"bob", []v1.SidebarCreate{{Type: "ticket", Label: "Ticket", Form: "new_ticket"}}},
		{"carol", nil},
	}
	for _, tc := range cases {
		t.Run(tc.user, func(t *testing.T) {
			resp, raw := sidebarFor(gateCtxFor(principalCtx(tc.user), t, d), t, app, "")
			require.Equal(t, tc.want, resp.Create)
			if tc.want == nil {
				require.NotContains(t, raw, "create")
			}
		})
	}

	// A type with no form is not offered even to a principal who may create it.
	installSpacesConfig(app, nil, map[string]dataentryconfig.Form{"new_ticket": {EntityType: "ticket"}},
		[]dataentryconfig.Space{{ID: "crm", Label: "CRM", Create: []string{"feature", "ticket"}}})
	resp, _ := sidebarFor(gateCtxFor(principalCtx("alice"), t, d), t, app, "")
	require.Equal(t, []v1.SidebarCreate{{Type: "ticket", Label: "Ticket", Form: "new_ticket"}}, resp.Create)
}

// A space's home is its `home:` entry, else the first destination of its
// navigation the principal can see, else /dashboard.
func TestSpaces_HomeDerivation(t *testing.T) {
	app := newTestAppV1(t)
	lists := map[string]dataentryconfig.List{"all": {EntityType: "ticket"}, "mine": {EntityType: "ticket"}}
	installSpacesConfig(app, lists, nil, []dataentryconfig.Space{
		{ID: "explicit", Label: "Explicit", Home: &dataentryconfig.NavigationEntry{Search: true},
			Navigation: []dataentryconfig.NavigationEntry{{Label: "All", List: "all"}}},
		{ID: "derived", Label: "Derived", Navigation: []dataentryconfig.NavigationEntry{
			{Label: "Run", Action: "run"},
			{Label: "Audit", List: "all", Permission: "admin:read"},
			{Group: "Work", Items: []dataentryconfig.NavigationEntry{{Label: "Mine", List: "mine"}}},
		}},
		{ID: "empty", Label: "Empty"},
		// A dashboard entry's href is "/"; as a home it would redirect to
		// the space's own root forever.
		{ID: "dash", Label: "Dash", Navigation: []dataentryconfig.NavigationEntry{
			{Label: "Dashboard", Dashboard: true},
		}},
	})
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	homes := func(user string) map[string]string {
		resp, _ := sidebarFor(gateCtxFor(principalCtx(user), t, d), t, app, "")
		out := map[string]string{}
		for _, sp := range resp.Spaces {
			out[sp.ID] = sp.Home
		}
		return out
	}
	require.Equal(t, map[string]string{"explicit": "/search", "derived": "/list/all", "empty": "/dashboard", "dash": "/dashboard"},
		homes("alice"))
	require.Equal(t, map[string]string{
		"explicit": "/search", "derived": "/list/mine", "empty": "/dashboard", "dash": "/dashboard",
	},
		homes("bob"), "the home skips an action and an entry the principal cannot see")
}

// statusSpaces is two spaces, each with one status entry inside a group.
func statusSpaces() []dataentryconfig.Space {
	status := []dataentryconfig.NavStatusRule{rule("info", "{count}", "")}
	return []dataentryconfig.Space{
		{ID: "crm", Label: "CRM", Navigation: []dataentryconfig.NavigationEntry{
			{Label: "Plain", List: "all"},
			{Group: "Work", Items: []dataentryconfig.NavigationEntry{
				{Label: "Plain", List: "all"},
				{Label: "Counted", List: "all", Status: status},
			}},
		}},
		{ID: "sales", Label: "Sales", Navigation: []dataentryconfig.NavigationEntry{
			{Label: "Counted", List: "all", Status: status},
		}},
	}
}

// Status keys of entries in a space carry the space id.
func TestSpaces_NavStatusKeysArePrefixed(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil, statusSpaces())

	keys := func(space string) []string {
		resp, _ := sidebarFor(t.Context(), t, app, space)
		var out []string
		for _, g := range resp.Navigation {
			for _, it := range g.Items {
				if it.StatusKey != "" {
					out = append(out, it.StatusKey)
				}
			}
		}
		return out
	}
	require.Equal(t, []string{"crm:1.1"}, keys("crm"))
	require.Equal(t, []string{"sales:0"}, keys("sales"))
}

// _nav_status counts only the resolved space's entries.
func TestSpaces_NavStatusLimitedToSpace(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T"}})
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil, statusSpaces())

	statusFor := func(space string) map[string]v1.NavStatus {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_nav_status?space="+space, http.NoBody).
			WithContext(principalCtx("alice"))
		rec := httptest.NewRecorder()
		handleV1NavStatus(app, rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp v1.NavStatusResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		return resp.Items
	}
	want := v1.NavStatus{Tone: "info", Label: "1", Count: 1}
	require.Equal(t, map[string]v1.NavStatus{"crm:1.1": want}, statusFor("crm"))
	require.Equal(t, map[string]v1.NavStatus{"sales:0": want}, statusFor("sales"))
	require.Equal(t, map[string]v1.NavStatus{"crm:1.1": want}, statusFor("nope"), "an unknown id resolves to the first")
}

// /_config serves every space, gated ones included, to every principal: the
// configuration is not a secret.
func TestSpaces_ConfigUnfiltered(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil, resolutionSpaces())
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody).
		WithContext(gateCtxFor(principalCtx("bob"), t, d))
	rec := httptest.NewRecorder()
	app.handleV1Config(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp v1.Config
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Spaces, 3)
	require.Equal(t, "admin:read", resp.Spaces[0].Permission)
}
