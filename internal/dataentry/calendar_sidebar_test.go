package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// installCalendarSidebarConfig publishes a Config carrying one calendar over
// tickets alongside a list, so a calendar entry can be compared against an
// entry that does carry a count.
func installCalendarSidebarConfig(app *App) {
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
	cfg.Navigation = []dataentryconfig.NavigationEntry{
		{Label: "All", List: "all-tickets"},
		{Label: "Schedule", Calendar: "schedule"},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// sidebarItemsByLabel returns every top-level sidebar item keyed by label,
// including those without a count (which sidebarCountsByLabel drops).
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
// icon are minted server-side, and — unlike a list or kanban — no count is
// attached.
//
// The absent count is the behavior under test, not an oversight. A list counts
// the set it displays; a calendar displays one period, so a total over all time
// could never agree with what is on screen and would never change as the user
// navigates. The sibling list entry is asserted to still carry a count, so a
// regression that drops counts wholesale cannot pass this test.
func TestSidebar_CalendarEntry(t *testing.T) {
	app := newTestAppV1(t)
	seedSidebarWorld(app)
	installCalendarSidebarConfig(app)

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
	if cal.Count != nil {
		t.Errorf("a calendar entry must carry no count, got %d", *cal.Count)
	}

	// Guard against a regression that simply stops emitting counts.
	if list, ok := items["All"]; !ok || list.Count == nil {
		t.Errorf("the sibling list entry must still carry a count, got %+v", list)
	}
}

// TestSidebar_CalendarHiddenWithoutPermission pins that a calendar entry is
// gated by the navigation entry's permission, exactly as other view kinds are —
// a calendar introduces no gating path of its own.
func TestSidebar_CalendarHiddenWithoutPermission(t *testing.T) {
	app := newTestAppV1(t)
	seedSidebarWorld(app)
	installCalendarSidebarConfig(app)

	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Navigation = []dataentryconfig.NavigationEntry{
		{Label: "Schedule", Calendar: "schedule", Permission: "view_schedule"},
	}
	next.Cfg = &cfg
	app.schema.Publish(&next)

	d := mustNewACL(t, sidebarPolicy(), app.store)
	app.acl = d

	items := sidebarItemsByLabel(gateCtxFor(aliceCtx(), t, d), t, app)
	if _, ok := items["Schedule"]; ok {
		t.Errorf("calendar entry must be hidden when its permission is not held: %+v", items)
	}
}
