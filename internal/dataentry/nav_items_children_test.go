package dataentry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// seedFeatureTree seeds three features and their tickets. alice reads every
// feature, and the tickets of project PRJ-42 only: TKT-003 is hidden.
func seedFeatureTree(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "Alpha"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "Beta"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-3", Type: "feature", Properties: map[string]any{"title": "Gamma"}})
	for _, tk := range []struct{ id, title, status, feat, prj string }{
		{"TKT-001", "One", "open", "FEAT-1", "PRJ-42"},
		{"TKT-002", "Two", "open", "FEAT-1", "PRJ-42"},
		{"TKT-003", "SECRET-ROW", "open", "FEAT-1", "PRJ-9"},
		{"TKT-004", "Four", "done", "FEAT-2", "PRJ-42"},
	} {
		seedEntity(app, &entity.Entity{ID: tk.id, Type: "ticket",
			Properties: map[string]any{"title": tk.title, "status": tk.status}})
		seedRelation(app, entity.NewRelation(tk.id, "implements", tk.feat))
		seedRelation(app, entity.NewRelation(tk.id, "belongs-to", tk.prj))
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"features": {Read: []string{"feature"}},
			"editor":   {Read: []string{"ticket"}},
		},
		Assignments:         map[string]string{"alice": "features"},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	setTestACL(t, app, d)
	return d
}

func installFeatureTree(app *App, from dataentryconfig.NavItemsFrom) {
	from.List = "features"
	installNavStatusConfig(app,
		map[string]dataentryconfig.List{
			"features": {EntityType: "feature", Sort: []dataentryconfig.SortSpec{{Property: "title"}}},
			"tickets":  {EntityType: "ticket", Sort: []dataentryconfig.SortSpec{{Property: "title"}}},
			"open": {EntityType: "ticket",
				Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}}},
		},
		[]dataentryconfig.NavigationEntry{{Group: "Features", ItemsFrom: &from}})
}

// Children are the child list's readable rows related to each entry, in the
// list's order; a hidden row is absent and does not count toward truncated.
func TestNavItems_ChildrenAreGatedAndGrouped(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedFeatureTree(t, app)
	installFeatureTree(app, dataentryconfig.NavItemsFrom{
		Children: &dataentryconfig.NavItemsChildren{List: "tickets", Via: "implements", Limit: 2},
	})

	items, body := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	got := items["0"].Entries
	require.Len(t, got, 3)
	require.Equal(t, []v1.NavItem{
		{ID: "TKT-001", Type: "ticket", Label: "One"},
		{ID: "TKT-002", Type: "ticket", Label: "Two"},
	}, got[0].Children)
	require.False(t, got[0].ChildrenTruncated, "the hidden third child does not make the entry truncated")
	require.Equal(t, "TKT-004", got[1].Children[0].ID)
	require.Empty(t, got[2].Children, "an entry with no edges has no children")
	require.NotContains(t, body, "SECRET-ROW")
	require.NotContains(t, body, "TKT-003")
}

// Children over the limit are capped and marked truncated.
func TestNavItems_ChildrenTruncated(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedFeatureTree(t, app)
	installFeatureTree(app, dataentryconfig.NavItemsFrom{
		Children: &dataentryconfig.NavItemsChildren{List: "tickets", Via: "implements", Limit: 1},
	})
	items, _ := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	require.Len(t, items["0"].Entries[0].Children, 1)
	require.True(t, items["0"].Entries[0].ChildrenTruncated)
}

// A count is the number of the counted list's readable rows related to the
// entry; hidden and filtered rows are not counted, and zero shows nothing.
func TestNavItems_StatusCountsGatedRows(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	d := seedFeatureTree(t, app)
	st := &dataentryconfig.NavItemsStatus{List: "open", Via: "implements", Tone: "info", Label: "{count} open"}
	installFeatureTree(app, dataentryconfig.NavItemsFrom{
		Status: st,
	})
	items, _ := navItemsAs(gateCtxFor(aliceCtx(), t, d), t, app)
	got := items["0"].Entries
	require.Equal(t, &v1.NavStatus{Tone: "info", Label: "2 open", Count: 2}, got[0].Status)
	require.Nil(t, got[1].Status, "only a done ticket: no count")
	require.Nil(t, got[2].Status, "no edges: no count")
}
