package dataentry

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Pages (TKT-ITQ0HL): a `page:` navigation entry is one sidebar row, the
// sidebar carries every page with the tabs the principal may see, and a tab's
// `permission:` is the same UX filter as a navigation entry's.

// installPagesConfig publishes a copy of the app's config with lists, pages
// and the top-level navigation set.
func installPagesConfig(
	app *App, lists map[string]dataentryconfig.List, pages map[string]dataentryconfig.Page,
	nav []dataentryconfig.NavigationEntry,
) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = make(map[string]dataentryconfig.List, len(cur.Cfg.Lists)+len(lists))
	maps.Copy(cfg.Lists, cur.Cfg.Lists)
	maps.Copy(cfg.Lists, lists)
	cfg.Pages = pages
	cfg.Navigation = nav
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// testPages is a page whose second tab is gated on admin:read, and a page
// whose only tab is.
func testPages() map[string]dataentryconfig.Page {
	return map[string]dataentryconfig.Page{
		"tickets": {Label: "Tickets", Tabs: []dataentryconfig.PageTab{
			{ID: "table", Label: "Table", List: "all"},
			{ID: "home", Label: "Home", Dashboard: true, Permission: "admin:read"},
		}},
		"admin": {Label: "Admin", Icon: "folder", Tabs: []dataentryconfig.PageTab{
			{ID: "all", Label: "All", List: "all", Permission: "admin:read"},
		}},
	}
}

func TestPages_Sidebar(t *testing.T) {
	app := newTestAppV1(t)
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, testPages(),
		[]dataentryconfig.NavigationEntry{
			{Page: "tickets"},
			{Label: "Admin work", Page: "admin"},
		})
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	t.Run("holder", func(t *testing.T) {
		resp, _ := sidebarFor(gateCtxFor(principalCtx("alice"), t, d), t, app, "")
		require.Equal(t, []v1.SidebarGroup{
			// The page's label, and the icon its first tab's kind derives.
			// Adjacent top-level entries share one unlabelled group.
			{Items: []v1.SidebarItem{
				{Label: "Tickets", Href: "/p/tickets", Icon: "list"},
				{Label: "Admin work", Href: "/p/admin", Icon: "folder"},
			}},
		}, resp.Navigation)
		require.Equal(t, v1.SidebarPage{Label: "Tickets", Tabs: []v1.SidebarPageTab{
			{ID: "table", Label: "Table", View: "list", Target: "all"},
			{ID: "home", Label: "Home", View: "dashboard"},
		}}, resp.Pages["tickets"])
	})

	t.Run("non-holder", func(t *testing.T) {
		resp, _ := sidebarFor(gateCtxFor(principalCtx("bob"), t, d), t, app, "")
		require.Equal(t, []string{"Tickets"}, navLabels(resp),
			"a page with no visible tab is left out of the sidebar")
		require.Equal(t, []v1.SidebarPageTab{{ID: "table", Label: "Table", View: "list", Target: "all"}},
			resp.Pages["tickets"].Tabs, "the gated tab is absent, the others remain")
		require.Empty(t, resp.Pages["admin"].Tabs)
	})
}

// Without `pages:` the sidebar carries no pages key.
func TestPages_NoPagesLeavesSidebarUnchanged(t *testing.T) {
	app := newTestAppV1(t)
	installNavStatusConfig(app,
		map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
		[]dataentryconfig.NavigationEntry{{Label: "All", List: "all"}})

	_, raw := sidebarFor(t.Context(), t, app, "")
	require.NotContains(t, raw, "pages")
}

// A page entry's status counts the rows of its first tab's list.
func TestPages_NavStatusCountsTheFirstTab(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "A"}})
	seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "B"}})
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, testPages(),
		[]dataentryconfig.NavigationEntry{{Page: "tickets", Status: []dataentryconfig.NavStatusRule{
			rule("info", "{count} tickets", ""),
		}}})

	resp, _ := sidebarFor(principalCtx("alice"), t, app, "")
	require.Equal(t, "0", resp.Navigation[0].Items[0].StatusKey)
	require.Equal(t, map[string]v1.NavStatus{"0": {Tone: "info", Label: "2 tickets", Count: 2}},
		navStatusAs(principalCtx("alice"), t, app))
}

// `_config` serves the pages verbatim, gated tabs included: the
// configuration is not a secret.
func TestPages_ConfigUnfiltered(t *testing.T) {
	app := newTestAppV1(t)
	installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, testPages(), nil)
	d := mustNewACL(t, gatedNavPolicy(), app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody).
		WithContext(gateCtxFor(principalCtx("bob"), t, d))
	rec := httptest.NewRecorder()
	app.handleV1Config(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp v1.Config
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Pages["tickets"].Tabs, 2)
	require.Equal(t, "admin:read", resp.Pages["admin"].Tabs[0].Permission)
}
