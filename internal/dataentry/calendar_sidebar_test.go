package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// installCalendarSidebarConfig publishes a Config carrying one calendar over
// tickets alongside a list, so a calendar entry can be compared against another
// entry kind.
func installCalendarSidebarConfig(app *App, nav []dataentryconfig.NavigationEntry) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = map[string]dataentryconfig.List{
		"all-tickets": {EntityType: "ticket", Title: "All"},
	}
	cfg.Calendars = map[string]dataentryconfig.Calendar{
		"schedule": {
			Title:   "Schedule",
			Sources: []dataentryconfig.CalendarSource{{EntityType: "ticket", Date: "due"}},
		},
	}
	cfg.Navigation = nav
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// sidebarItemsByLabel performs a sidebar request and returns items by label.
func sidebarItemsByLabel(ctx context.Context, t *testing.T, app *App) map[string]v1.SidebarItem {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v\nbody: %s", err, rec.Body)
	}
	out := make(map[string]v1.SidebarItem)
	for _, group := range resp.Navigation {
		for _, item := range group.Items {
			out[item.Label] = item
		}
	}
	return out
}

// TestSidebar_CalendarEntry covers the calendar navigation arm: the href and
// icon are minted server-side, the same way every other entry kind's are.
func TestSidebar_CalendarEntry(t *testing.T) {
	app := newTestAppV1(t)
	installCalendarSidebarConfig(app, []dataentryconfig.NavigationEntry{
		{Label: "All", List: "all-tickets"},
		{Label: "Schedule", Calendar: "schedule"},
	})

	items := sidebarItemsByLabel(context.Background(), t, app)

	cal, ok := items["Schedule"]
	if !ok {
		t.Fatalf("calendar entry missing from sidebar: %+v", items)
	}
	if cal.Href != "/calendar/schedule" {
		t.Errorf("href = %q, want /calendar/schedule", cal.Href)
	}
	if cal.Icon != "calendar" {
		t.Errorf("icon = %q, want calendar", cal.Icon)
	}
	// The sibling list entry proves the sidebar rendered normally rather than
	// the calendar arm being the only thing exercised.
	if _, ok := items["All"]; !ok {
		t.Errorf("list entry missing: %+v", items)
	}
}

// TestSidebar_CalendarHiddenWithoutPermission pins that a calendar entry is
// gated by the navigation entry's permission, exactly as other view kinds are —
// a calendar introduces no gating path of its own.
func TestSidebar_CalendarHiddenWithoutPermission(t *testing.T) {
	app := newTestAppV1(t)
	installCalendarSidebarConfig(app, []dataentryconfig.NavigationEntry{
		{Label: "Schedule", Calendar: "schedule", Permission: "view_schedule"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	items := sidebarItemsByLabel(gateCtxFor(aliceCtx(), t, d), t, app)
	if _, ok := items["Schedule"]; ok {
		t.Errorf("calendar entry must be hidden when its permission is not held: %+v", items)
	}
}
