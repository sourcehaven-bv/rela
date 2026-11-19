package dataentry

import (
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A permission granted by a role a role relation confers counts for the
// gates that only decide what to render (TKT-CBRLZWU): carla holds
// `watcher` on TKT-1 through `watches`, and watcher grants admin:read. She
// sees the gated nav entries, space and dashboard card, as alice (a global
// holder) does. bob, who holds no such edge, still does not. A command gated
// on the same permission, a document (elevated or not), and the built-in
// history:read the role also grants, stay global-only.
func TestRoleRelationPermission_ShowsGatedUI(t *testing.T) {
	cmd := CommandConfig{Script: "x.lua", Context: "entity", Permission: "admin:read"}
	for _, tc := range []struct {
		user                string
		ui, command, global bool
	}{
		{user: "alice", ui: true, command: true, global: true},
		{user: "carla", ui: true, command: false, global: false},
		{user: "bob", ui: false, command: false, global: false},
	} {
		t.Run(tc.user, func(t *testing.T) {
			app, d := roleRelationPermissionApp(t)
			authz, err := newGatedAuthorizer(d)
			require.NoError(t, err)

			installGatedNavConfig(app)
			ctx := gateCtxFor(principalCtx(tc.user), t, d)
			require.Equal(t, tc.ui, slices.Contains(sidebarLabels(ctx, t, app), "Audit"), "nav entry")

			installGatedDashboardConfig(app)
			require.Equal(t, tc.ui, slices.Contains(dashboardTitles(ctx, t, app), "Audit"), "dashboard card")

			installSpacesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nil,
				[]dataentryconfig.Space{
					{ID: "open", Label: "Open", Navigation: []dataentryconfig.NavigationEntry{{Label: "All", List: "all"}}},
					{
						ID: "portal", Label: "Portal", Permission: "admin:read",
						Navigation: []dataentryconfig.NavigationEntry{{Label: "All", List: "all"}},
					},
				})
			resp, _ := sidebarFor(ctx, t, app, "")
			require.Equal(t, tc.ui, slices.Contains(spaceIDs(resp), "portal"), "space")

			installPagesConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, testPages(),
				[]dataentryconfig.NavigationEntry{{Page: "tickets"}})
			resp, _ = sidebarFor(ctx, t, app, "")
			wantTabs := 1
			if tc.ui {
				wantTabs = 2
			}
			require.Len(t, resp.Pages["tickets"].Tabs, wantTabs, "page tab")

			installDashboardCards(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}},
				dataentryconfig.DashboardCard{Title: "All", List: "all", Display: "count", Permission: "admin:read"})
			wantCode := http.StatusNotFound
			if tc.ui {
				wantCode = http.StatusOK
			}
			rec := dashboardCardAs(ctx, t, app, "0")
			require.Equal(t, wantCode, rec.Code, "dashboard card value: %s", rec.Body.String())

			require.Equal(t, tc.command, authz.Authorize(ctx, cmd), "a command counts global roles only")
			doc := dataentryconfig.DocumentConfig{Permission: "admin:read"}
			require.Equal(t, tc.command, gateDocumentPermission(ctx, "audit", doc) == nil,
				"a document counts global roles only")
			doc.AllowACLBypass = metamodel.ACLBypassRead
			require.Equal(t, tc.command, authorizeElevatedDocument(ctx, d, doc),
				"an elevated document counts global roles only")
			gate := readGateFromContext(ctx)
			require.Equal(t, tc.global, gate.HoldsPermission(ctx, "admin:read"),
				"the global-only check is unchanged")
			require.False(t, gate.HoldsPermission(ctx, acl.PermHistoryRead),
				"a built-in permission from a role relation is not granted everywhere")
		})
	}
}

// roleRelationPermissionApp is an app whose policy is gatedNavPolicy plus a
// `watches` role relation that confers watcher, which grants admin:read and
// history:read.
// carla watches TKT-1.
func roleRelationPermissionApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "carla", Type: "person", Properties: map[string]any{"title": "Carla"}})
	seedRelation(app, entity.NewRelation("carla", "watches", "TKT-1"))
	p := gatedNavPolicy()
	p.Roles["watcher"] = acl.RoleDef{Read: []string{"ticket"}, Permissions: []string{"admin:read", acl.PermHistoryRead}}
	p.Roles["staff"] = acl.RoleDef{Permissions: []string{"watchers.manage"}}
	p.RoleRelations = map[string]acl.RoleRelationDef{
		"watches": {Confers: "watcher", RequiresPermission: "watchers.manage"},
	}
	d := mustNewACL(t, p, app.store)
	setTestACL(t, app, d)

	return app, d
}
