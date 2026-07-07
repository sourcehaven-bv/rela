package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func feedHandlerApp(t *testing.T, feeds map[string]dataentryconfig.Feed, tasks ...*entity.Entity) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label: "Task", IDPrefix: "TSK-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"due":    {Type: metamodel.PropertyTypeDate},
					"status": {Type: metamodel.PropertyTypeString},
				},
				PropertyOrder: []string{"title", "due", "status"},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Test"},
		Forms:      map[string]dataentryconfig.Form{},
		Lists:      map[string]dataentryconfig.List{},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Feeds:      feeds,
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	f := newFixture()
	for _, e := range tasks {
		f.AddNode(e)
	}
	return newAppFromParts(cfg, meta, f)
}

func task(id, title, status, due string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "task", Properties: map[string]any{
		"title": title, "status": status, "due": due,
	}}
}

// doFeed drives the full router (so middleware + ACL gate + CSRF exemption all
// run), mimicking a calendar poller: no Origin, no Cookie, no Sec-Fetch-Site.
func doFeed(t *testing.T, app *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

func TestFeedHandler_ICS(t *testing.T) {
	app := feedHandlerApp(t,
		map[string]dataentryconfig.Feed{
			"tasks": {
				Meta:    dataentryconfig.FeedMeta{Name: "PIM tasks"},
				Sources: []dataentryconfig.FeedSource{{EntityType: "task", Where: []string{"status != done"}, Date: "due", Summary: "title"}},
			},
		},
		task("TSK-1", "Renew passport", "todo", "2026-07-10"),
		task("TSK-2", "Finished", "done", "2026-07-11"),
	)

	rec := doFeed(t, app, "/api/v1/_feeds/tasks.ics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("content-type = %q, want text/calendar", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "BEGIN:VCALENDAR") || !strings.Contains(body, "END:VCALENDAR") {
		t.Errorf("not a VCALENDAR:\n%s", body)
	}
	if !strings.Contains(body, "SUMMARY:Renew passport") {
		t.Errorf("missing expected event; body:\n%s", body)
	}
	if !strings.Contains(body, "UID:task--TSK-1@rela") {
		t.Errorf("missing/incorrect UID; body:\n%s", body)
	}
	// Deep link must be ABSOLUTE (scheme://host) so a calendar client can open
	// it — a relative path is useless once the .ics leaves the server.
	if !strings.Contains(body, "URL:http://localhost/entity/task/TSK-1") {
		t.Errorf("expected absolute deep link; body:\n%s", body)
	}
	if strings.Contains(body, "Finished") {
		t.Errorf("status filter failed: 'done' task leaked into feed")
	}
	if !strings.Contains(body, "X-WR-CALNAME:PIM tasks") {
		t.Errorf("missing calendar name")
	}
}

func TestFeedHandler_JSON(t *testing.T) {
	app := feedHandlerApp(t,
		map[string]dataentryconfig.Feed{
			"tasks": {Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}},
		},
		task("TSK-1", "A task", "todo", "2026-07-10"),
	)

	rec := doFeed(t, app, "/api/v1/_feeds/tasks.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var out struct {
		Events []struct {
			UID, Summary, Date string
			AllDay             bool
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v; body=%s", err, rec.Body.String())
	}
	if len(out.Events) != 1 || out.Events[0].Summary != "A task" || out.Events[0].Date != "2026-07-10" {
		t.Errorf("json events wrong: %+v", out.Events)
	}
	if !out.Events[0].AllDay {
		t.Error("event should be all-day")
	}
}

// TestFeedPath_CSRFExemption pins the exemption logic directly (the security
// middleware is not wired in the lightweight test app, so this exercises
// isCSRFExempt itself). A bare poller — no Sec-Fetch-Site, Cookie, or Origin —
// is exempt on the feed path; a browser-shaped request is not.
func TestFeedPath_CSRFExemption(t *testing.T) {
	feedPath := "/api/v1/_feeds/tasks.ics"

	// The feed prefix is a non-browser-exempt path.
	if !isSensitivePath(feedPath) {
		t.Error("feed path should be sensitive (under /api/)")
	}

	// Bare poller: exempt.
	poller := httptest.NewRequest(http.MethodGet, feedPath, http.NoBody)
	if !isCSRFExempt(poller) {
		t.Error("a bare poller request to the feed path should be CSRF-exempt")
	}

	// Browser (Sec-Fetch-Site present): NOT exempt.
	browser := httptest.NewRequest(http.MethodGet, feedPath, http.NoBody)
	browser.Header.Set("Sec-Fetch-Site", "cross-site")
	if isCSRFExempt(browser) {
		t.Error("a browser request (Sec-Fetch-Site set) must NOT be CSRF-exempt")
	}

	// Credentialed (Cookie present): NOT exempt.
	cookied := httptest.NewRequest(http.MethodGet, feedPath, http.NoBody)
	cookied.Header.Set("Cookie", "session=abc")
	if isCSRFExempt(cookied) {
		t.Error("a credentialed request must NOT be CSRF-exempt")
	}
}

func TestFeedHandler_PollerSucceeds(t *testing.T) {
	// A bare poller request (no Origin/Cookie/Sec-Fetch-Site) succeeds.
	app := feedHandlerApp(t,
		map[string]dataentryconfig.Feed{"tasks": {Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}}},
		task("TSK-1", "A", "todo", "2026-07-10"),
	)
	rec := doFeed(t, app, "/api/v1/_feeds/tasks.ics")
	if rec.Code != http.StatusOK {
		t.Fatalf("poller request rejected: status %d", rec.Code)
	}
}

func TestFeedHandler_NotFound(t *testing.T) {
	app := feedHandlerApp(t, map[string]dataentryconfig.Feed{
		"tasks": {Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}},
	})
	cases := []struct {
		path string
		want int
	}{
		{"/api/v1/_feeds/unknown.ics", http.StatusNotFound}, // unknown feed
		{"/api/v1/_feeds/tasks.txt", http.StatusNotFound},   // unsupported ext
		{"/api/v1/_feeds/tasks", http.StatusNotFound},       // no ext
		{"/api/v1/_feeds/a/b.ics", http.StatusNotFound},     // slash in name
	}
	for _, tc := range cases {
		rec := doFeed(t, app, tc.path)
		if rec.Code != tc.want {
			t.Errorf("GET %s = %d, want %d", tc.path, rec.Code, tc.want)
		}
	}
}

func TestFeedHandler_EmptyFeedIsValidCalendar(t *testing.T) {
	app := feedHandlerApp(t, map[string]dataentryconfig.Feed{
		"tasks": {Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}},
	})
	rec := doFeed(t, app, "/api/v1/_feeds/tasks.ics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "BEGIN:VCALENDAR") || !strings.Contains(body, "END:VCALENDAR") {
		t.Errorf("empty feed is not a valid VCALENDAR:\n%s", body)
	}
	if strings.Contains(body, "BEGIN:VEVENT") {
		t.Error("empty feed contains an event")
	}
}
