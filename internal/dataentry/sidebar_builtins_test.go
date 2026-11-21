package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// The built-in sidebar rows (Search, Inbox, Flow tasks, Flow runs, Analysis)
// and the as-of picker follow `builtins:`: per space with `spaces:`, else at the top level. A row's
// `permission:` is the same UX filter as a navigation entry's.

// portalBuiltins hides four rows in a portal space and gates Analysis on
// admin:read in a staff space.
func portalBuiltins() []dataentryconfig.Space {
	return []dataentryconfig.Space{
		{ID: "staff", Label: "Staff",
			Navigation: []dataentryconfig.NavigationEntry{{Label: "All", List: "all"}},
			Builtins: dataentryconfig.Builtins{
				dataentryconfig.BuiltinAnalysis: {Permission: "admin:read"},
			}},
		{ID: "portal", Label: "Portal",
			Navigation: []dataentryconfig.NavigationEntry{{Label: "Mine", List: "all"}},
			Builtins: dataentryconfig.Builtins{
				dataentryconfig.BuiltinAnalysis:  {Hidden: true},
				dataentryconfig.BuiltinFlowRuns:  {Hidden: true},
				dataentryconfig.BuiltinSearch:    {Hidden: true},
				dataentryconfig.BuiltinFlowTasks: {Hidden: true},
				dataentryconfig.BuiltinAsOf:      {Hidden: true},
			}},
	}
}

func TestSidebarBuiltins_Spaces(t *testing.T) {
	app := newTestAppV1(t)
	installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil, portalBuiltins())
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	setTestACL(t, app, d)

	all := []dataentryconfig.BuiltinRow{
		dataentryconfig.BuiltinSearch, dataentryconfig.BuiltinFlowTasks,
		dataentryconfig.BuiltinFlowRuns, dataentryconfig.BuiltinAnalysis,
		dataentryconfig.BuiltinAsOf,
	}
	cases := []struct {
		name  string
		user  string
		space string
		want  []dataentryconfig.BuiltinRow
	}{
		{"holder keeps the gated row", "alice", "staff", nil},
		{"non-holder loses the gated row", "bob", "staff",
			[]dataentryconfig.BuiltinRow{dataentryconfig.BuiltinAnalysis}},
		{"hidden rows are hidden for a holder", "alice", "portal", all},
		{"hidden rows are hidden for a non-holder", "bob", "portal", all},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, raw := sidebarFor(gateCtxFor(principalCtx(tc.user), t, d), t, app, tc.space)
			require.Equal(t, tc.space, resp.Space)
			require.Equal(t, tc.want, resp.HiddenBuiltins, "rows come back in sidebar order")
			if tc.want == nil {
				require.NotContains(t, raw, "hidden_builtins")
			}
		})
	}
}

// installTopLevelBuiltins publishes a config with one list in the top-level
// navigation and the given `builtins:`.
func installTopLevelBuiltins(app *App, builtins dataentryconfig.Builtins) {
	installNavStatusConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{{Label: "All", List: "all"}})
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Builtins = builtins
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

func TestSidebarBuiltins_TopLevel(t *testing.T) {
	gated := dataentryconfig.Builtins{dataentryconfig.BuiltinInbox: {Permission: "admin:read"}}
	cases := []struct {
		name     string
		builtins dataentryconfig.Builtins
		user     string
		want     []dataentryconfig.BuiltinRow
	}{
		{"no block shows every row", nil, "bob", nil},
		{"hidden row", dataentryconfig.Builtins{dataentryconfig.BuiltinSearch: {Hidden: true}}, "alice",
			[]dataentryconfig.BuiltinRow{dataentryconfig.BuiltinSearch}},
		{"gated row for a holder", gated, "alice", nil},
		{"gated row for a non-holder", gated, "bob", []dataentryconfig.BuiltinRow{dataentryconfig.BuiltinInbox}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			installTopLevelBuiltins(app, tc.builtins)
			d := mustNewACL(t, gatedNavPolicy(), app.store)
			setTestACL(t, app, d)

			resp, raw := sidebarFor(gateCtxFor(principalCtx(tc.user), t, d), t, app, "")
			require.Equal(t, tc.want, resp.HiddenBuiltins)
			require.Equal(t, []string{"All"}, navLabels(resp), "the navigation is unchanged")
			require.NotContains(t, raw, "space", "no space fields without spaces:")
		})
	}
}

// Without acl.yaml a gated row is shown, like a gated navigation entry.
func TestSidebarBuiltins_NopACLShowsGatedRow(t *testing.T) {
	app := newTestAppV1(t)
	installTopLevelBuiltins(app, dataentryconfig.Builtins{dataentryconfig.BuiltinAnalysis: {Permission: "admin:read"}})
	setTestACL(t, app, acl.NopACL{})

	resp, _ := sidebarFor(principalCtx("bob"), t, app, "")
	require.Empty(t, resp.HiddenBuiltins)
}

// `_config` serves the `builtins:` blocks unfiltered to every principal: the
// configuration is not a secret, and the per-principal answer is the
// sidebar's hidden_builtins.
func TestSidebarBuiltins_ConfigUnfiltered(t *testing.T) {
	gated := dataentryconfig.Builtins{dataentryconfig.BuiltinAnalysis: {Permission: "admin:read"}}
	cases := []struct {
		name    string
		install func(app *App)
		get     func(v1.Config) dataentryconfig.Builtins
	}{
		{"top level", func(app *App) { installTopLevelBuiltins(app, gated) },
			func(c v1.Config) dataentryconfig.Builtins { return c.Builtins }},
		{"space", func(app *App) {
			installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil,
				[]dataentryconfig.Space{{ID: "staff", Label: "Staff", Builtins: gated}})
		}, func(c v1.Config) dataentryconfig.Builtins { return c.Spaces[0].Builtins }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			tc.install(app)
			d := mustNewACL(t, gatedNavPolicy(), app.store)
			setTestACL(t, app, d)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody).
				WithContext(gateCtxFor(principalCtx("bob"), t, d))
			rec := httptest.NewRecorder()
			app.handleV1Config(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			var resp v1.Config
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, gated, tc.get(resp), "a non-holder sees the gate too")
		})
	}
}
