package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// installNav publishes a navigation tree on the app's current config.
func installNav(app *App, nav []dataentryconfig.NavigationEntry) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Navigation = nav
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// sidebarEntityItems returns every `entities:` item the sidebar serves.
func sidebarEntityItems(ctx context.Context, t *testing.T, app *App) []v1.SidebarItem {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v", err)
	}
	var out []v1.SidebarItem
	for _, g := range resp.Navigation {
		for _, it := range g.Items {
			if it.Entities != nil {
				out = append(out, it)
			}
		}
	}
	return out
}

// listQueryFor renders a served `entities:` definition as the list request
// the SPA sends for it (SidebarEntityQuery.vue).
func listQueryFor(e *v1.SidebarEntities) string {
	q := url.Values{}
	if e.QueryScope != "" {
		q.Set(QueryScopeParam, e.QueryScope)
	}
	if e.Sort != "" {
		q.Set("sort", e.Sort)
	}
	q.Set("per_page", "100")
	return q.Encode()
}

// TestSidebarEntities_ServesDefinitionOnly pins the wire shape: the sidebar
// carries the query, never the rows, and no href or label of its own.
func TestSidebarEntities_ServesDefinitionOnly(t *testing.T) {
	app := newScopeTestApp(t)
	installNav(app, []dataentryconfig.NavigationEntry{{Group: "Werk", Items: []dataentryconfig.NavigationEntry{
		{Entities: "taak", QueryScope: "mijn", Sort: []dataentryconfig.SortSpec{{Property: "title", Direction: "desc"}}},
		{Entities: "taak"},
	}}})

	items := sidebarEntityItems(context.Background(), t, app)
	if len(items) != 2 {
		t.Fatalf("got %d entities items, want 2", len(items))
	}
	want := []v1.SidebarEntities{
		{Type: "taak", QueryScope: "mijn", Sort: "-title"},
		{Type: "taak"},
	}
	for i, it := range items {
		if !reflect.DeepEqual(*it.Entities, want[i]) {
			t.Errorf("item %d entities = %+v, want %+v", i, *it.Entities, want[i])
		}
		if it.Href != "" || it.Label != "" {
			t.Errorf("item %d: href %q / label %q, want both empty — the rows supply them", i, it.Href, it.Label)
		}
		if it.Icon != dataentryconfig.DerivedIconNames.Entity {
			t.Errorf("item %d icon = %q, want the derived entity glyph", i, it.Icon)
		}
	}
}

// TestSidebarEntities_RowsFollowThePrincipal drives the served definition
// through the list endpoint, as the SPA does, for a scope reading
// current_user. The sidebar payload is identical for both users; the rows
// are not.
func TestSidebarEntities_RowsFollowThePrincipal(t *testing.T) {
	app := newScopeTestApp(t)
	installNav(app, []dataentryconfig.NavigationEntry{{Group: "Mijn werk", Items: []dataentryconfig.NavigationEntry{
		{Entities: "taak", QueryScope: "mijn"},
	}}})

	items := sidebarEntityItems(context.Background(), t, app)
	if len(items) != 1 {
		t.Fatalf("got %d entities items, want 1", len(items))
	}
	query := listQueryFor(items[0].Entities)

	for user, want := range map[string]string{"alice": "TAAK-1,TAAK-3", "bob": "TAAK-2"} {
		rec := scopeListAs(app, user, query)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: list %d %s", user, rec.Code, rec.Body)
		}
		if got := strings.Join(idsFromListBody(t, rec), ","); got != want {
			t.Errorf("%s: rows = %s, want %s", user, got, want)
		}
	}
}

// TestSidebarEntities_RowsAreACLGated pins that a principal without a read
// grant on the type gets the same served entry and an empty row set — the
// same answer as a type with no matches.
func TestSidebarEntities_RowsAreACLGated(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"ticket"}},
			"lead":   {Read: []string{"ticket", "feature"}},
		},
		Assignments: map[string]string{"alice": "viewer", "bob": "lead"},
	}, app.store)
	app.acl = d
	installNav(app, []dataentryconfig.NavigationEntry{{Group: "Features", Items: []dataentryconfig.NavigationEntry{
		{Entities: "feature"},
	}}})

	aliceItems := sidebarEntityItems(gateCtxFor(aliceCtx(), t, d), t, app)
	bobItems := sidebarEntityItems(gateCtxFor(principalCtx("bob"), t, d), t, app)
	if !reflect.DeepEqual(aliceItems, bobItems) {
		t.Fatalf("sidebar differs by principal: alice %+v, bob %+v", aliceItems, bobItems)
	}

	query := listQueryFor(aliceItems[0].Entities)
	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "feature", "features", query)
	if rec.Code != http.StatusOK || len(resp.Data) != 0 || resp.Meta.Total != 0 {
		t.Errorf("alice (no grant): code %d, %d rows, total %d; want 200 with nothing",
			rec.Code, len(resp.Data), resp.Meta.Total)
	}
	resp, rec = listEntitiesAs(principalCtx("bob"), t, app, d, "feature", "features", query)
	if rec.Code != http.StatusOK || len(resp.Data) != 1 {
		t.Errorf("bob (granted): code %d, %d rows; want the one feature", rec.Code, len(resp.Data))
	}
}
