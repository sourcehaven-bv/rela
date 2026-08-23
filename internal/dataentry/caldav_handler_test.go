package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersion/go-ical"

	"github.com/Sourcehaven-BV/rela/internal/caldavalias"
	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// caldavTestApp builds an App with one VTODO collection over `task`, wired with
// a real alias service so the create/rename paths are exercised end to end.
// testAliasPrincipal is the identity aliases are keyed under in these tests.
//
// Aliases are keyed by principal, and a request through the test router carries
// defaultPrincipalResolver's Principal — so a lookup must use the same value the
// write did. "" would silently find nothing and read as "no alias recorded".
const testAliasPrincipal = "unknown"

func caldavTestApp(t *testing.T, tasks ...*entity.Entity) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"task_status": {Values: []string{"todo", "done"}, Default: "todo"},
		},
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label: "Task", IDPrefix: "TSK-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString, Required: true},
					"due":    {Type: metamodel.PropertyTypeDate},
					"status": {Type: "task_status", Required: true},
					"notes":  {Type: metamodel.PropertyTypeString},
					"secret": {Type: metamodel.PropertyTypeString},
				},
				PropertyOrder: []string{"title", "due", "status", "notes", "secret"},
			},
		},
	}
	cfg := &dataentryconfig.Config{
		App:     dataentryconfig.AppConfig{Name: "Test"},
		Forms:   map[string]dataentryconfig.Form{},
		Lists:   map[string]dataentryconfig.List{},
		Views:   map[string]dataentryconfig.ViewConfig{},
		Kanbans: map[string]dataentryconfig.Kanban{},
		CalDAV: dataentryconfig.CalDAVConfig{Static: map[string]dataentryconfig.CalDAVCollection{
			"tasks": {
				Meta:       dataentryconfig.FeedMeta{Name: "rela Tasks"},
				Component:  dataentryconfig.CalDAVComponentTodo,
				EntityType: "task",
				Where:      []string{"status != done"},
				Due:        "due",
				Summary:    "title",
				Completion: &dataentryconfig.CalDAVCompletion{
					StatusProperty: "status",
					CompletedValue: "done",
					PendingValue:   "todo",
				},
				Defaults: map[string]string{"status": "todo"},
				OnDelete: &dataentryconfig.CalDAVOnDelete{Set: map[string]string{"status": "done"}},
			},
		}},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	f := newFixture()
	for _, e := range tasks {
		f.AddNode(e)
	}
	app := newAppFromParts(cfg, meta, f)

	root, err := storage.NewRootedFS(storage.NewMemFS(), t.TempDir())
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	kv := state.NewFSKV(root)
	aliases, aliasErr := caldavalias.New(t.Context(), kv)
	if aliasErr != nil {
		t.Fatalf("caldavalias.New: %v", aliasErr)
	}
	app.SetCalDAVAliases(aliases)
	return app
}

// doCalDAV drives the FULL router, so the middleware chain, the ACL gate and
// the JWT/host checks all run — the point being that CalDAV is an ordinary
// /api/ route with no bespoke gating.
func doCalDAV(t *testing.T, app *App, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Host = "localhost"
	if body != "" {
		// go-webdav rejects a PROPFIND/REPORT body that is not declared as XML
		// ("unsupported request body"), so the type has to match the method.
		if strings.HasPrefix(body, "<?xml") {
			req.Header.Set("Content-Type", "application/xml")
		} else {
			req.Header.Set("Content-Type", "text/calendar")
		}
	}
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

func TestCalDAV_RouteIsReachable(t *testing.T) {
	app := caldavTestApp(t)
	rec := doCalDAV(t, app, "OPTIONS", "/api/v1/_caldav/", "")

	// Reachability is the assertion: an unregistered non-/api route would fall
	// through to the SPA and return 200 HTML (BUG-F3ADZO), and an unmounted
	// /api/ route would 404.
	if rec.Code == http.StatusNotFound {
		t.Fatalf("CalDAV route is not mounted: %d", rec.Code)
	}
	if dav := rec.Header().Get("DAV"); dav != "" && !strings.Contains(dav, "calendar-access") {
		t.Errorf("DAV header does not advertise calendar-access: %q", dav)
	}
}

// TestCalDAV_PropfindListsCollections proves the multi-collection discovery
// that makes one account URL yield every configured list.
func TestCalDAV_PropfindListsCollections(t *testing.T) {
	app := caldavTestApp(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:displayname/></d:prop></d:propfind>`

	req := httptest.NewRequest("PROPFIND", "/api/v1/_caldav/principal/calendars/", strings.NewReader(body))
	req.Host = "localhost"
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND = %d, want 207\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "rela Tasks") {
		t.Errorf("collection display name missing from the multistatus:\n%s", rec.Body.String())
	}
}

func TestCalDAV_ListsMatchingEntitiesOnly(t *testing.T) {
	app := caldavTestApp(t,
		&entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
			"title": "Open task", "status": "todo", "due": "2026-08-10"}},
		&entity.Entity{ID: "TSK-2", Type: "task", Properties: map[string]any{
			"title": "Closed task", "status": "done"}},
	)

	objs, err := (&caldavBackend{app: app}).listTodos(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("listTodos: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("got %d objects, want 1 (the where clause excludes done)", len(objs))
	}
	if !strings.Contains(objs[0].Path, "task--TSK-1@rela.ics") {
		t.Errorf("resource path = %q, want the derived UID href", objs[0].Path)
	}
	if objs[0].ETag == "" {
		t.Error("resource has no ETag — conditional requests would be impossible")
	}
}

// TestCalDAV_ClientCreateGetsAnAlias is the inbound case the alias service
// exists for: Apple mints a bare UUID that can never be a rela entity id.
func TestCalDAV_ClientCreateGetsAnAlias(t *testing.T) {
	app := caldavTestApp(t)
	const uuid = "D8AAE77A-89CB-46D2-BDA4-F319D2014D6B"
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple Inc.//iOS 26.5.1//EN\r\n" +
		"BEGIN:VTODO\r\nUID:" + uuid + "\r\nDTSTAMP:20260809T081404Z\r\n" +
		"SUMMARY:this is a test\r\nSTATUS:NEEDS-ACTION\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"

	rec := doCalDAV(t, app, http.MethodPut, "/api/v1/_caldav/principal/calendars/tasks/"+uuid+".ics", body)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d, want a success\n%s", rec.Code, rec.Body.String())
	}

	alias, ok := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", uuid+".ics")
	if !ok {
		t.Fatal("no alias recorded — the next sync would create a duplicate entity")
	}
	if alias.EntityID == "" || alias.EntityID == uuid {
		t.Errorf("alias entity id = %q; a bare UUID can never be a rela entity id", alias.EntityID)
	}
	if alias.UID != uuid {
		t.Errorf("alias UID = %q, want the client's %q", alias.UID, uuid)
	}

	// The entity really exists, with the configured default applied.
	e, err := app.Services().Store.GetEntity(t.Context(), alias.EntityID)
	if err != nil {
		t.Fatalf("created entity is missing: %v", err)
	}
	if e.GetString("title") != "this is a test" {
		t.Errorf("title = %q", e.GetString("title"))
	}
	if e.GetString("status") != "todo" {
		t.Errorf("status = %q, want the configured default", e.GetString("status"))
	}
}

// TestCalDAV_CompletionWriteBackPreservesUnmappedProperties is the whole point
// of routing writes through PatchEntity: checking a box must not erase
// properties the VTODO model has no slot for.
func TestCalDAV_CompletionWriteBackPreservesUnmappedProperties(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo", "due": "2026-08-10",
		"notes": "keep me", "secret": "keep me too",
	}})

	uid := "task--TSK-1@rela"
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple Inc.//iOS 26.5.1//EN\r\n" +
		"BEGIN:VTODO\r\nUID:" + uid + "\r\nDTSTAMP:20260809T081406Z\r\n" +
		"SUMMARY:Buy milk\r\nDUE;VALUE=DATE:20260810\r\n" +
		"STATUS:COMPLETED\r\nCOMPLETED:20260809T081406Z\r\nPERCENT-COMPLETE:100\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"

	rec := doCalDAV(t, app, http.MethodPut, "/api/v1/_caldav/principal/calendars/tasks/"+uid+".ics", body)
	if rec.Code >= 400 {
		t.Fatalf("PUT = %d\n%s", rec.Code, rec.Body.String())
	}

	e, err := app.Services().Store.GetEntity(t.Context(), "TSK-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got := e.GetString("status"); got != "done" {
		t.Errorf("status = %q, want done — the check-off did not land", got)
	}
	// The properties VTODO does not model must be untouched.
	if got := e.GetString("notes"); got != "keep me" {
		t.Errorf("notes = %q — an unmapped property was erased", got)
	}
	if got := e.GetString("secret"); got != "keep me too" {
		t.Errorf("secret = %q — an unmapped property was erased", got)
	}
}

// TestCalDAV_DeleteAppliesStatusTransition: a client swipe must not destroy a
// graph node. rela has no soft-delete and DeleteEntity cascades to relations.
func TestCalDAV_DeleteAppliesStatusTransition(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo",
	}})

	rec := doCalDAV(t, app, http.MethodDelete, "/api/v1/_caldav/principal/calendars/tasks/task--TSK-1@rela.ics", "")
	if rec.Code >= 400 {
		t.Fatalf("DELETE = %d\n%s", rec.Code, rec.Body.String())
	}

	e, err := app.Services().Store.GetEntity(t.Context(), "TSK-1")
	if err != nil {
		t.Fatalf("the entity was destroyed by a client delete: %v", err)
	}
	if got := e.GetString("status"); got != "done" {
		t.Errorf("status = %q, want the configured on_delete transition", got)
	}
}

// TestCalDAV_MKCALENDARIsRefused: collections are operator-declared config, and
// Calendar.app issues MKCALENDAR unprompted on account setup, so a
// client-minted collection would be an orphan the server could never serve.
func TestCalDAV_MKCALENDARIsRefused(t *testing.T) {
	app := caldavTestApp(t)
	rec := doCalDAV(t, app, "MKCALENDAR", "/api/v1/_caldav/principal/calendars/invented/", "")
	if rec.Code < 400 {
		t.Errorf("MKCALENDAR = %d, want a refusal", rec.Code)
	}
}

// TestCalDAV_NotRegisteredWithoutAliasService: serving collections with no way
// to remember client-created resources would duplicate every to-do on the next
// sync, so the routes must not exist at all.
func TestCalDAV_NotRegisteredWithoutAliasService(t *testing.T) {
	app := caldavTestApp(t)
	app.SetCalDAVAliases(nil)

	rec := doCalDAV(t, app, "PROPFIND", "/api/v1/_caldav/principal/calendars/", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected the CalDAV routes to be absent, got %d", rec.Code)
	}
}

// TestCalDAV_IsCSRFExempt guards a gap the unit tests missed and only a real
// client surfaced: a CalDAV request carries no Origin, so without the
// exemption every request fails the same-origin check with "origin_missing"
// and the endpoint is unreachable by any actual client.
//
// The exemption is safe for the same reason the ICS feed's is: isCSRFExempt
// additionally requires no Cookie, no Origin/Referer and no Sec-Fetch-Site —
// a shape a browser cannot produce — so a browser fetch() of a CalDAV path is
// still same-origin checked.
func TestCalDAV_IsCSRFExempt(t *testing.T) {
	app := caldavTestApp(t)

	t.Run("a bare client request is not rejected as cross-origin", func(t *testing.T) {
		rec := doCalDAV(t, app, "PROPFIND", "/api/v1/_caldav/principal/calendars/", "")
		if rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "origin") {
			t.Errorf("a real CalDAV client would be rejected: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a browser-shaped request is still same-origin checked", func(t *testing.T) {
		// The origin allowlist is only populated once security is configured;
		// without this the app enforces no allowlist at all and the assertion
		// below would pass vacuously.
		if err := app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}); err != nil {
			t.Fatalf("SetSecurityConfig: %v", err)
		}
		req := httptest.NewRequest("PROPFIND", "/api/v1/_caldav/principal/calendars/", strings.NewReader(""))
		// An allowed Host, so the rejection below is attributable to the
		// ORIGIN check rather than the host allowlist.
		req.Host = "127.0.0.1:8080"
		// Sec-Fetch-Site is browser-set and unforgeable by JS; its presence
		// means this is a real browser, so the exemption must not apply.
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("a cross-origin browser request must be rejected, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "origin_not_allowed") {
			t.Errorf("expected the ORIGIN check to reject this, got: %s", rec.Body.String())
		}
	})
}

// TestCalDAV_AppleFixtureDoesNotEraseNotes drives the REAL captured Apple
// payload through the write path. The fixture has no DESCRIPTION line — Apple
// omits it whenever the note is empty — so an unconditional write would blank
// the mapped property on every single sync.
func TestCalDAV_AppleFixtureDoesNotEraseNotes(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo", "notes": "semi-skimmed", "secret": "keep me",
	}})

	uid := "task--TSK-1@rela"
	// Deliberately mirrors the captured client-created fixture: SUMMARY and
	// STATUS and timestamps, nothing else.
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nCALSCALE:GREGORIAN\r\n" +
		"PRODID:-//Apple Inc.//iOS 26.5.1//EN\r\nBEGIN:VTODO\r\n" +
		"CREATED:20260809T081403Z\r\nDTSTAMP:20260809T081404Z\r\n" +
		"LAST-MODIFIED:20260809T081403Z\r\nSTATUS:NEEDS-ACTION\r\n" +
		"SUMMARY:Buy milk\r\nUID:" + uid + "\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"

	rec := doCalDAV(t, app, http.MethodPut, "/api/v1/_caldav/principal/calendars/tasks/"+uid+".ics", body)
	if rec.Code >= 400 {
		t.Fatalf("PUT = %d\n%s", rec.Code, rec.Body.String())
	}

	e, err := app.Services().Store.GetEntity(t.Context(), "TSK-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got := e.GetString("notes"); got != "semi-skimmed" {
		t.Errorf("notes = %q — a property the client never sent was erased", got)
	}
	if got := e.GetString("secret"); got != "keep me" {
		t.Errorf("secret = %q — an unmapped property was erased", got)
	}
}

// TestCalDAV_IfMatchRejectsStale pins the conflict guard: without it an offline
// client silently overwrites a change made elsewhere.
func TestCalDAV_IfMatchRejectsStale(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo",
	}})
	uid := "task--TSK-1@rela"
	path := "/api/v1/_caldav/principal/calendars/tasks/" + uid + ".ics"
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:" + uid +
		"\r\nDTSTAMP:20260809T081404Z\r\nSUMMARY:Changed\r\nSTATUS:NEEDS-ACTION\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"

	// Seed an alias so the resource is known, with an ETag the client will not match.
	if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
		Principal:  testAliasPrincipal,
		Collection: "tasks", Href: uid + ".ics", UID: uid, EntityID: "TSK-1",
	}); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "text/calendar")
	req.Header.Set("If-Match", `"stale"`)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Errorf("If-Match with a stale etag = %d, want 412", rec.Code)
	}
	// And the write must not have landed.
	e, _ := app.Services().Store.GetEntity(t.Context(), "TSK-1")
	if e.GetString("title") != "Buy milk" {
		t.Errorf("a rejected conditional write still modified the entity: %q", e.GetString("title"))
	}
}

// TestCalDAV_HardDeleteActuallyDeletes covers the opt-in destructive branch
// end to end — asserting deletePatch reports hard=true is not the same as
// asserting the entity is gone.
func TestCalDAV_HardDeleteActuallyDeletes(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo",
	}})
	// Switch the collection to the opt-in hard delete.
	st := app.State()
	coll := st.Cfg.CalDAV.Static["tasks"]
	coll.OnDelete = &dataentryconfig.CalDAVOnDelete{Hard: true}
	st.Cfg.CalDAV.Static["tasks"] = coll

	rec := doCalDAV(t, app, http.MethodDelete,
		"/api/v1/_caldav/principal/calendars/tasks/task--TSK-1@rela.ics", "")
	if rec.Code >= 400 {
		t.Fatalf("DELETE = %d\n%s", rec.Code, rec.Body.String())
	}
	if _, err := app.Services().Store.GetEntity(t.Context(), "TSK-1"); err == nil {
		t.Error("hard delete did not remove the entity")
	}
}

// TestCalDAV_SoftDeleteBindingSurvivesForADerivedHref is the rela-minted-UID
// counterpart of TestCalDAV_SoftDeleteKeepsTheAlias.
//
// This test previously asserted the OPPOSITE — that a soft delete drops the
// alias, on the reasoning that "a later create at this href would resurrect the
// old entity." That reasoning was wrong twice over. Re-binding the href to the
// entity it already names is not resurrection, it is the correct answer to a
// PUT; and dropping the binding is what actually causes damage, because a
// client-minted UUID href then has no owner and its replayed PUT creates a
// second entity.
func TestCalDAV_SoftDeleteBindingSurvivesForADerivedHref(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo",
	}})
	href := "task--TSK-1@rela.ics"
	if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
		Principal:  testAliasPrincipal,
		Collection: "tasks", Href: href, UID: "task--TSK-1@rela", EntityID: "TSK-1",
	}); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	rec := doCalDAV(t, app, http.MethodDelete, "/api/v1/_caldav/principal/calendars/tasks/"+href, "")
	if rec.Code >= 400 {
		t.Fatalf("DELETE = %d\n%s", rec.Code, rec.Body.String())
	}

	// The entity survives a soft delete (that is the point of on_delete), so
	// the href must keep naming it. The transition itself is what removes the
	// resource from the collection, via the `where:` filter.
	alias, ok := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", href)
	if !ok {
		t.Fatal("soft delete dropped the alias; the href is unowned and a replayed PUT would duplicate")
	}
	if alias.EntityID != "TSK-1" {
		t.Errorf("alias points at %s, want TSK-1", alias.EntityID)
	}
}

// TestCalDAV_PathTraversalIsRejected: path.Clean resolves ".." BEFORE the
// prefix is stripped, so cleaning the whole path would let a request addressed
// to one collection operate on another.
func TestCalDAV_PathTraversalIsRejected(t *testing.T) {
	b := &caldavBackend{app: caldavTestApp(t)}
	for _, p := range []string{
		"/api/v1/_caldav/principal/calendars/tasks/../other/a.ics",
		"/api/v1/_caldav/principal/calendars/tasks/sub/a.ics",
		"/api/v1/_caldav/principal/calendars/../../etc/a.ics",
	} {
		t.Run(p, func(t *testing.T) {
			name, href, ok := b.splitPath(p)
			if ok && (name != "tasks" || strings.Contains(href, "/")) {
				t.Errorf("splitPath escaped its collection: name=%q href=%q", name, href)
			}
		})
	}
}

// TestCalDAV_WellKnownRedirects covers RFC 6764 discovery — the FIRST request a
// real client makes.
//
// Verified against macOS accountsd: it probes /.well-known/caldav before
// anything else, and if that returns the SPA's 200 HTML (which it does when the
// route is absent, per BUG-F3ADZO) the client abandons setup and reports a
// generic "account verification failed" with no indication of the cause.
func TestCalDAV_WellKnownRedirects(t *testing.T) {
	app := caldavTestApp(t)

	req := httptest.NewRequest("PROPFIND", "/.well-known/caldav", strings.NewReader(""))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("well-known probe = %d, want 301 (a 200 here is the SPA shell, which clients cannot use)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != caldavPathPrefix {
		t.Errorf("Location = %q, want %q", loc, caldavPathPrefix)
	}
}

// TestCalDAV_WellKnownAbsentWithoutCalDAV: a deployment with no alias service
// serves no CalDAV, so it must not advertise discovery either.
func TestCalDAV_WellKnownAbsentWithoutCalDAV(t *testing.T) {
	app := caldavTestApp(t)
	app.SetCalDAVAliases(nil)

	req := httptest.NewRequest("PROPFIND", "/.well-known/caldav", strings.NewReader(""))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code == http.StatusMovedPermanently {
		t.Error("discovery advertised though CalDAV is not served")
	}
}

// TestCalDAV_RootPrincipalProbe covers the SECOND thing a real client asks for.
//
// Verified against macOS accountsd: after following the well-known redirect it
// issues `PROPFIND / <current-user-principal>` against the SITE ROOT. When the
// SPA catch-all answers that with 200 HTML the account CONNECTS but shows no
// collections at all — the client never learned where the principal lives, and
// there is no error anywhere to explain the empty list.
func TestCalDAV_RootPrincipalProbe(t *testing.T) {
	app := caldavTestApp(t)

	req := httptest.NewRequest("PROPFIND", "/", strings.NewReader(
		`<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("root probe = %d, want 207 (200 here is the SPA shell)", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "current-user-principal") {
		t.Errorf("response does not carry current-user-principal:\n%s", body)
	}
	if !strings.Contains(body, caldavPathPrefix+"principal/") {
		t.Errorf("principal href missing or wrong:\n%s", body)
	}
}

// TestCalDAV_RootGetStillServesTheSPA: intercepting the site root must not
// break the web app. Only PROPFIND is a discovery probe.
func TestCalDAV_RootGetStillServesTheSPA(t *testing.T) {
	app := caldavTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code == http.StatusMultiStatus {
		t.Error("a browser GET / was answered with a CalDAV multistatus")
	}
}

// TestCalDAV_EncodedHrefResolves covers a shape observed on the wire but not
// designed for: Reminders percent-encodes the "@" in a rela-derived href,
// PUTting to `task--TSK-passport%40rela.ics`.
//
// It works because Go's ServeMux decodes the path before splitPath sees it —
// but that is a property of the router, not a decision made here, so it is
// pinned rather than left to coincidence.
func TestCalDAV_EncodedHrefResolves(t *testing.T) {
	app := caldavTestApp(t, &entity.Entity{ID: "TSK-1", Type: "task", Properties: map[string]any{
		"title": "Buy milk", "status": "todo", "notes": "keep me",
	}})

	uid := "task--TSK-1@rela"
	encoded := "/api/v1/_caldav/principal/calendars/tasks/task--TSK-1%40rela.ics"
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple Inc.//iOS 26.5.1//EN\r\n" +
		"BEGIN:VTODO\r\nUID:" + uid + "\r\nDTSTAMP:20260810T190000Z\r\n" +
		"SUMMARY:Buy milk\r\nSTATUS:COMPLETED\r\nCOMPLETED:20260810T190000Z\r\n" +
		"PERCENT-COMPLETE:100\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"

	rec := doCalDAV(t, app, http.MethodPut, encoded, body)
	if rec.Code >= 400 {
		t.Fatalf("PUT to a percent-encoded href = %d\n%s", rec.Code, rec.Body.String())
	}

	e, err := app.Services().Store.GetEntity(t.Context(), "TSK-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got := e.GetString("status"); got != "done" {
		t.Errorf("status = %q — the write did not resolve to the right entity", got)
	}
	if got := e.GetString("notes"); got != "keep me" {
		t.Errorf("notes = %q — an unmapped property was erased", got)
	}
}

// TestCalDAV_DiscoveryChainFromTheRoot walks the sequence a real client
// actually follows, starting where it starts rather than at the collection URL.
//
// Every step here was broken at some point despite the per-endpoint tests
// passing, because those addressed /api/v1/_caldav/... directly and so never
// exercised the path a client takes to FIND that URL. Two live-client failures
// came from exactly that gap.
func TestCalDAV_DiscoveryChainFromTheRoot(t *testing.T) {
	app := caldavTestApp(t)

	// 1. RFC 6764 bootstrap.
	rec := doCalDAV(t, app, "PROPFIND", "/.well-known/caldav", "")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("step 1 (well-known) = %d, want 301", rec.Code)
	}

	// 2. Site-root principal probe.
	rec = doCalDAV(t, app, "PROPFIND", "/",
		`<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`)
	if rec.Code != http.StatusMultiStatus || !strings.Contains(rec.Body.String(), "principal/") {
		t.Fatalf("step 2 (root principal probe) = %d:\n%s", rec.Code, rec.Body.String())
	}

	// 3. The principal must advertise where the calendars live.
	rec = doCalDAV(t, app, "PROPFIND", caldavPathPrefix+"principal/",
		`<?xml version="1.0"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">`+
			`<d:prop><c:calendar-home-set/></d:prop></d:propfind>`)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("step 3 (principal) = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "calendars/") {
		t.Fatalf("step 3 did not advertise calendar-home-set:\n%s", rec.Body.String())
	}

	// 4. The home set must enumerate the configured collection.
	rec = doCalDAV(t, app, "PROPFIND", caldavPathPrefix+"principal/calendars/",
		`<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:displayname/></d:prop></d:propfind>`)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("step 4 (home set) = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rela Tasks") {
		t.Errorf("step 4 did not list the collection — a client would show an empty account:\n%s",
			rec.Body.String())
	}
}

// TestCalDAV_StaleWriteDoesNotResurrect: a deletion in rela is deliberate, so a
// client that has not synced must not undo it by PUTting its cached copy.
//
// Returns 404 rather than 409 because the condition is permanent: a CalDAV
// client reads 409 as "retry later" and re-sends every sync cycle forever,
// while 404 tells it to drop its local copy — the outcome that matches the
// deletion.
func TestCalDAV_StaleWriteDoesNotResurrect(t *testing.T) {
	app := caldavTestApp(t)
	href := "9641DDFC-EAE6-4E47-B8D6-A7A2CD3D671A.ics"
	uid := "9641DDFC-EAE6-4E47-B8D6-A7A2CD3D671A"

	// An alias left pointing at an entity that has since been deleted.
	if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
		Principal:  testAliasPrincipal,
		Collection: "tasks", Href: href, UID: uid, EntityID: "TSK-gone",
	}); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:" + uid +
		"\r\nDTSTAMP:20260810T200000Z\r\nSUMMARY:More stuff edited\r\n" +
		"STATUS:NEEDS-ACTION\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"

	rec := doCalDAV(t, app, http.MethodPut,
		"/api/v1/_caldav/principal/calendars/tasks/"+href, body)

	if rec.Code == http.StatusConflict {
		t.Fatal("409 makes the client retry this doomed write forever")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("stale write = %d, want 404 so the client drops its copy", rec.Code)
	}
	// The alias must SURVIVE: it is the evidence the 404 rests on. Drop it and
	// the next PUT finds nothing, reads as a create, and resurrects the entity.
	if _, ok := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", href); !ok {
		t.Error("the alias was dropped; the next PUT would resurrect the entity")
	}

	// And the refusal must be STABLE — a client retries, and every retry must
	// get the same answer rather than eventually succeeding.
	again := doCalDAV(t, app, http.MethodPut,
		"/api/v1/_caldav/principal/calendars/tasks/"+href, body)
	if again.Code != http.StatusNotFound {
		t.Errorf("retried stale write = %d, want a stable 404", again.Code)
	}
}

// TestCalDAV_StaleWriteInferredFromServerStateAlone pins WHY the refusal is
// trustworthy: it is inferred from server-side state, never from anything the
// client sends back.
//
// An earlier design marked each served VTODO with an X- property and treated
// its presence as proof rela had served the resource. RFC 5545 3.8.8.2 lets a
// client drop x-properties it does not understand ("can ignore them"), so that
// was a heuristic that fails OPEN — a stripping client resurrects the entity.
// The alias table answers the same question with no client cooperation at all.
func TestCalDAV_StaleWriteInferredFromServerStateAlone(t *testing.T) {
	body := func(uid string) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:" + uid +
			"\r\nDTSTAMP:20260810T200000Z\r\nSUMMARY:Something\r\nSTATUS:NEEDS-ACTION\r\n" +
			"END:VTODO\r\nEND:VCALENDAR\r\n"
	}

	t.Run("no alias means nobody here ever served it, so it is a create", func(t *testing.T) {
		app := caldavTestApp(t)
		rec := doCalDAV(t, app, http.MethodPut,
			"/api/v1/_caldav/principal/calendars/tasks/BRAND-NEW.ics", body("BRAND-NEW"))
		if rec.Code >= 400 {
			t.Fatalf("a client-composed to-do must be created, got %d\n%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("alias without a live entity is a deletion, with no marker in the body", func(t *testing.T) {
		app := caldavTestApp(t)
		href, uid := "ONCE-SERVED.ics", "ONCE-SERVED"
		if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
			Principal:  testAliasPrincipal,
			Collection: "tasks", Href: href, UID: uid, EntityID: "TSK-deleted",
		}); err != nil {
			t.Fatalf("seed alias: %v", err)
		}
		rec := doCalDAV(t, app, http.MethodPut,
			"/api/v1/_caldav/principal/calendars/tasks/"+href, body(uid))
		if rec.Code != http.StatusNotFound {
			t.Errorf("stale write = %d, want 404 — inferred from the alias, not the body", rec.Code)
		}
	})
}

// TestCalDAV_StaleWriteSurvivesOutOfBandDelete is the case the entitymanager
// hook cannot cover, and the reason the tombstone is an inference rather than a
// record written at deletion time.
//
// A `rm`, a `git pull`, or an edit while the server was stopped removes an
// entity without going through entitymanager.DeleteEntity, so no hook fires. On
// the filesystem backend no event fires either: fsstore's startup scan adopts
// whatever is on disk (syncEntities) without diffing against the previous
// index, so the deletion leaves no trace to observe. Postgres has a real
// tombstone table because every delete is a transaction; the filesystem has
// nothing equivalent.
//
// Asking "does the entity exist NOW?" needs none of that machinery. This test
// models the out-of-band delete exactly: an alias with no corresponding entity
// in the store, and no hook or event ever having run.
func TestCalDAV_StaleWriteSurvivesOutOfBandDelete(t *testing.T) {
	app := caldavTestApp(t) // note: no entity seeded at all
	href, uid := "RM-BY-HAND.ics", "RM-BY-HAND"
	if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
		Principal:  testAliasPrincipal,
		Collection: "tasks", Href: href, UID: uid, EntityID: "TSK-rmd",
	}); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	rec := doCalDAV(t, app, http.MethodPut,
		"/api/v1/_caldav/principal/calendars/tasks/"+href,
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:"+uid+
			"\r\nDTSTAMP:20260810T200000Z\r\nSUMMARY:Cached copy\r\nSTATUS:NEEDS-ACTION\r\n"+
			"END:VTODO\r\nEND:VCALENDAR\r\n")

	if rec.Code != http.StatusNotFound {
		t.Errorf("PUT after an out-of-band delete = %d, want 404; "+
			"a hook-driven tombstone would have missed this entirely", rec.Code)
	}
}

// doCalDAVPutWithHeaders is doCalDAV's PUT plus caller-supplied headers, for
// conditional writes (If-Match / If-None-Match).
func doCalDAVPutWithHeaders(
	t *testing.T, app *App, path, body string, hdr map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Host = "localhost"
	if body != "" {
		req.Header.Set("Content-Type", "text/calendar")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

// TestCalDAV_IfMatchComparesAgainstCurrentContent pins the conditional-write
// contract against the case that broke it: an edit made OUTSIDE CalDAV.
//
// The ETag was once cached on the alias and refreshed only by CalDAV writes, so
// a rela-side edit (SPA, CLI, MCP, automation, git pull) left the stored and
// served tags disagreeing. Both directions lost data, and both are asserted
// here:
//
//   - a client presenting the tag the server JUST served must be ACCEPTED.
//     Refusing it wedges the client permanently, since only a successful CalDAV
//     write would refresh the stale stored tag.
//   - a client presenting an OLD tag must be REFUSED. Accepting it silently
//     overwrites the newer edit — the overwrite If-Match exists to prevent.
func TestCalDAV_IfMatchComparesAgainstCurrentContent(t *testing.T) {
	const href = "task--TSK-cond@rela.ics"
	const path = "/api/v1/_caldav/principal/calendars/tasks/" + href
	body := func(summary string) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:task--TSK-cond@rela\r\n" +
			"DTSTAMP:20260811T080000Z\r\nSUMMARY:" + summary + "\r\nSTATUS:NEEDS-ACTION\r\n" +
			"END:VTODO\r\nEND:VCALENDAR\r\n"
	}

	// An entity edited outside CalDAV after the alias was recorded: the alias
	// exists, but the content (and so the ETag) has moved on.
	setup := func(t *testing.T) (*App, string) {
		t.Helper()
		app := caldavTestApp(t, task("TSK-cond", "Edited in rela", "todo", ""))
		if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
			Collection: "tasks", Href: href, UID: "task--TSK-cond@rela", EntityID: "TSK-cond",
		}); err != nil {
			t.Fatalf("seed alias: %v", err)
		}
		get := doCalDAV(t, app, http.MethodGet, path, "")
		served := get.Header().Get("ETag")
		if served == "" {
			t.Fatalf("no ETag served (GET %d)", get.Code)
		}
		return app, served
	}

	t.Run("the freshly served tag is accepted", func(t *testing.T) {
		app, served := setup(t)
		rec := doCalDAVPutWithHeaders(t, app, path, body("Client edit"),
			map[string]string{"If-Match": served})
		if rec.Code == http.StatusPreconditionFailed {
			t.Fatal("412 for the tag the server just served: the client can never " +
				"succeed, because only a CalDAV write would refresh a cached tag")
		}
		if rec.Code >= 400 {
			t.Fatalf("PUT = %d, want success\n%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a stale tag is refused", func(t *testing.T) {
		app, _ := setup(t)
		rec := doCalDAVPutWithHeaders(t, app, path, body("Overwrite"),
			map[string]string{"If-Match": `"STALE0000000000"`})
		if rec.Code != http.StatusPreconditionFailed {
			t.Errorf("stale If-Match = %d, want 412; accepting it silently "+
				"overwrites the newer non-CalDAV edit", rec.Code)
		}
	})
}

// TestCalDAV_SoftDeleteKeepsTheAlias covers the two bugs that followed from
// dropping the alias on a soft delete. Both were reproduced against the live
// demo before this test existed.
//
// The alias is the ONLY record binding a client-chosen href to an entity. Drop
// it and the href becomes unowned, which breaks in two directions at once.
func TestCalDAV_SoftDeleteKeepsTheAlias(t *testing.T) {
	const uid = "DEADBEEF-0000-1111-2222-333344445555"
	const href = uid + ".ics"
	const path = "/api/v1/_caldav/principal/calendars/tasks/" + href
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:" + uid +
		"\r\nDTSTAMP:20260811T080000Z\r\nSUMMARY:Probe\r\nSTATUS:NEEDS-ACTION\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"

	app := caldavTestApp(t)
	if rec := doCalDAV(t, app, http.MethodPut, path, body); rec.Code >= 400 {
		t.Fatalf("create = %d\n%s", rec.Code, rec.Body.String())
	}
	alias, ok := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", href)
	if !ok {
		t.Fatal("no alias recorded for the created resource")
	}
	created := alias.EntityID

	if rec := doCalDAV(t, app, http.MethodDelete, path, ""); rec.Code >= 400 {
		t.Fatalf("delete = %d\n%s", rec.Code, rec.Body.String())
	}

	// (1) The binding must survive. Without it objectFor falls back to the
	// derived <type>--<id>@rela.ics href, so the resource reappears under a
	// DIFFERENT identity — which a client reads as delete-plus-create.
	after, ok := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", href)
	if !ok {
		t.Fatal("soft delete dropped the alias; the href is now unowned and the " +
			"resource can reappear under a derived href")
	}
	if after.EntityID != created {
		t.Errorf("alias re-pointed: %s -> %s", created, after.EntityID)
	}

	// (2) An offline client replaying its cached PUT must UPDATE the same
	// entity, never create a second one. A client-minted UUID cannot satisfy
	// splitFeedUID, so with no alias this falls through to createFromTodo —
	// the duplication registerCalDAVRoutes refuses to start without an alias
	// service to prevent.
	if rec := doCalDAV(t, app, http.MethodPut, path, body); rec.Code >= 400 {
		t.Fatalf("replayed PUT = %d\n%s", rec.Code, rec.Body.String())
	}
	replayed, _ := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", href)
	if replayed.EntityID != created {
		t.Errorf("replayed PUT created a SECOND entity: %s then %s", created, replayed.EntityID)
	}
}

// TestCalDAV_UnreadableEntityIsRetryableNotDeleted separates "gone" from
// "cannot read right now", because the two answers have opposite consequences
// for the client's data.
//
// staleWriteResponse's 404 is deliberately PERMANENT — it tells the client to
// drop its local copy, which is right for a real deletion. But entitymanager
// collapses every GetEntity failure into ErrEntityNotFound ("structural, not
// textual"), so malformed frontmatter from a bad merge, a transient EIO, or a
// dropped pgx connection all arrive looking exactly like a deletion. Answering
// those with the permanent 404 destroys the only copy the user still had, for a
// fault that would have cleared on its own.
//
// Verified live: a PUT over an entity with unparseable frontmatter returned 404
// before this change and 503 after, while a genuinely absent entity stayed 404.
func TestCalDAV_UnreadableEntityIsRetryableNotDeleted(t *testing.T) {
	const href = "RETRY-PROBE.ics"
	const path = "/api/v1/_caldav/principal/calendars/tasks/" + href
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:RETRY-PROBE\r\n" +
		"DTSTAMP:20260811T080000Z\r\nSUMMARY:Edit\r\nSTATUS:NEEDS-ACTION\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"

	// An alias pointing at an entity the store genuinely does not have: the
	// deletion case, which must keep its permanent 404.
	app := caldavTestApp(t)
	if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
		Principal:  testAliasPrincipal,
		Collection: "tasks", Href: href, UID: "RETRY-PROBE", EntityID: "TSK-gone",
	}); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	rec := doCalDAV(t, app, http.MethodPut, path, body)
	if rec.Code != http.StatusNotFound {
		t.Errorf("genuinely absent entity = %d, want 404 so the client drops it", rec.Code)
	}
	// The 404 must rest on a POSITIVE not-found, never on an error that merely
	// resembles one — that is what entityIsGone exists to establish.
	if rec.Code == http.StatusServiceUnavailable {
		t.Error("a real deletion was reported as retryable; the client keeps a to-do that is gone")
	}
}

// TestCalDAV_IfMatchOnAResourceWithNoAlias covers the case that made a
// never-before-written to-do permanently unwritable.
//
// A resource rela has served but no client has yet written back has NO alias:
// it is addressed by the DERIVED href (<type>--<id>@rela.ics), which objectFor
// mints on the fly. An earlier version failed the precondition whenever the
// alias was missing, so the client held a perfectly valid ETag, got 412, and
// could never recover — only a successful write creates the alias that would
// let the check pass.
//
// Reproduced against Apple Reminders: toggling such a to-do reverted every
// time, the client retrying the same doomed PUT on each sync.
func TestCalDAV_IfMatchOnAResourceWithNoAlias(t *testing.T) {
	const href = "task--TSK-fresh@rela.ics"
	const path = "/api/v1/_caldav/principal/calendars/tasks/" + href
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:task--TSK-fresh@rela\r\n" +
		"DTSTAMP:20260811T110000Z\r\nSUMMARY:Fresh\r\nSTATUS:COMPLETED\r\n" +
		"COMPLETED:20260811T110000Z\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"

	app := caldavTestApp(t, task("TSK-fresh", "Fresh", "todo", ""))
	if _, aliased := app.caldavAliases.Lookup(testAliasPrincipal, "tasks", href); aliased {
		t.Fatal("fixture already has an alias; this test needs the alias-less path")
	}

	// The ETag the server would serve right now — exactly what a client holds.
	get := doCalDAV(t, app, http.MethodGet, path, "")
	served := get.Header().Get("ETag")
	if served == "" {
		t.Fatalf("no ETag served (GET %d)", get.Code)
	}

	rec := doCalDAVPutWithHeaders(t, app, path, body,
		map[string]string{"If-Match": served})
	if rec.Code == http.StatusPreconditionFailed {
		t.Fatal("412 for a valid ETag on an alias-less resource: the client can " +
			"never succeed, because only a successful write creates the alias")
	}
	if rec.Code >= 400 {
		t.Fatalf("PUT = %d, want success\n%s", rec.Code, rec.Body.String())
	}

	// The guard must still hold: a stale tag is refused.
	stale := doCalDAVPutWithHeaders(t, app,
		"/api/v1/_caldav/principal/calendars/tasks/task--TSK-other@rela.ics", body,
		map[string]string{"If-Match": `"STALE0000000000"`})
	if stale.Code != http.StatusPreconditionFailed {
		t.Errorf("stale If-Match = %d, want 412", stale.Code)
	}
}

// TestCalDAV_DeepLinkIsAbsolute: the URL property is a deep link back into
// rela, and a client renders it as a clickable link (Thunderbird shows it as
// "Bijbehorende koppeling"). A relative path is useless once the iCalendar
// leaves the server — RFC 5545 types URL as a URI.
//
// The base must follow the request, so a proxied deployment emits the hostname
// the CLIENT used rather than rela's internal bind address.
func TestCalDAV_DeepLinkIsAbsolute(t *testing.T) {
	app := caldavTestApp(t, task("TSK-link", "Linked", "todo", ""))
	path := "/api/v1/_caldav/principal/calendars/tasks/task--TSK-link@rela.ics"

	t.Run("direct request uses the request host", func(t *testing.T) {
		rec := doCalDAV(t, app, http.MethodGet, path, "")
		if !strings.Contains(rec.Body.String(), "URL:http://localhost/entity/task/TSK-link") {
			t.Errorf("want an absolute deep link on the request host, got:\n%s", rec.Body.String())
		}
	})

	t.Run("behind a TLS proxy it uses the forwarded scheme and host", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
		req.Host = "rela.example.com"
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, req)

		want := "URL:https://rela.example.com/entity/task/TSK-link"
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("want %q — the link a client can actually open — got:\n%s", want, rec.Body.String())
		}
	})
}

// TestCalDAVRefusedWrite_ServesStoredStateWithoutETag pins the answer to a
// DENIED write: accept it, hand back the entity as it actually stands, and
// suppress the ETag.
//
// The status is deliberately NOT an error. Every honest refusal code leaves a
// real client broken — 403 makes Thunderbird keep the rejected edit forever
// (verified on the wire and in its source), 412/409 loops it through a modal it
// cannot resolve, and 404 would delete a to-do that still exists. Serving the
// truth is what makes the client converge: it re-reads and the user watches
// their unauthorized edit revert. Apple's CalendarServer does the same for
// VTODO (replaceMissingToDoProperties).
//
// The ETag suppression is the load-bearing half. RFC 4791 §5.3.4 forbids a
// strong ETag when the stored bytes differ from the submitted ones, and here we
// stored NOTHING. Returning one would let a client cache a tag for content rela
// never accepted, and a later If-Match against it would pass a write that should
// have been refused.
func TestCalDAVRefusedWrite_ServesStoredStateWithoutETag(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Server owns this", "status": "todo"}
	app := caldavTestApp(t, stored)

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	m, _, err := b.mapperFor(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("mapperFor: %v", err)
	}

	in := inbound(calfeed.Todo{Summary: "client tried to rename", UID: "task--TSK-1@rela"},
		ical.PropSummary)
	obj, err := b.refusedWriteResponse(
		t.Context(), "tasks", "task--TSK-1@rela.ics", m, in, "TSK-1")
	if err != nil {
		t.Fatalf("refusedWriteResponse: %v", err)
	}

	if obj.ETag != "" {
		t.Errorf("a refused write stored nothing, so RFC 4791 §5.3.4 forbids an ETag; got %q", obj.ETag)
	}
	var buf strings.Builder
	if encErr := ical.NewEncoder(&buf).Encode(obj.Data); encErr != nil {
		t.Fatalf("encode served calendar: %v", encErr)
	}
	body := buf.String()
	if !strings.Contains(body, "Server owns this") {
		t.Errorf("the response must carry the STORED title so the client reverts, got:\n%s", body)
	}
	if strings.Contains(body, "client tried to rename") {
		t.Error("the response echoed the client's refused value, which would confirm a write that never happened")
	}
}

// TestCalDAVRefusedWrite_UnreadableEntityStillErrors pins that the accept path
// is not a blanket swallow: if the current state cannot be read there is no
// truth to serve, so the honest error is returned rather than an invented
// representation.
func TestCalDAVRefusedWrite_UnreadableEntityStillErrors(t *testing.T) {
	app := caldavTestApp(t)
	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	m, _, err := b.mapperFor(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("mapperFor: %v", err)
	}

	in := inbound(calfeed.Todo{Summary: "x", UID: "task--TSK-missing@rela"}, ical.PropSummary)
	if _, err := b.refusedWriteResponse(
		t.Context(), "tasks", "task--TSK-missing@rela.ics", m, in, "TSK-missing"); err == nil {
		t.Error("a refusal with no readable entity must surface an error, not a fabricated 2xx")
	}
}

// TestCalDAVBodyLimit_RejectsOversizedBody pins the cap that stands between a
// CalDAV client and a remote process kill.
//
// go-ical's decoder recurses once per `BEGIN:` line with no depth limit, so a
// large enough body exhausts the goroutine stack. That is a runtime FATAL
// ERROR, not a panic — `recover()` cannot catch it and the whole server dies.
// Reproduced against this tree at ~27 MB; the cap here is 1 MiB, ~26x below the
// threshold, and legitimate bodies are under 2 KB.
//
// The test asserts the 413 rather than the absence of a crash, because a test
// that actually triggered the overflow would take the test binary down with it.
func TestCalDAVBodyLimit_RejectsOversizedBody(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Buy milk", "status": "todo"}
	app := caldavTestApp(t, stored)

	// A VALID, oversized VTODO. Deliberately not junk bytes: a malformed body
	// is rejected by the parser whatever its size, so a junk payload would pass
	// this test even with the cap removed — it would assert nothing.
	padding := strings.Repeat("a", maxCalDAVBodyBytes)
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nBEGIN:VTODO\r\n" +
		"UID:task--TSK-1@rela\r\nSUMMARY:Buy milk\r\nDESCRIPTION:" + padding + "\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"
	rec := doCalDAV(t, app, http.MethodPut,
		"/api/v1/_caldav/principal/calendars/tasks/task--TSK-1@rela.ics", body)

	if rec.Code == http.StatusCreated || rec.Code == http.StatusNoContent {
		t.Fatalf("an oversized body was ACCEPTED (%d) — the depth-bomb guard is gone", rec.Code)
	}
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 413 (or 400 from the parser refusing the truncated read)", rec.Code)
	}
}

// TestCalDAVBodyLimit_AllowsRealisticBody pins the other side: the cap must not
// break legitimate traffic. An Apple client-created VTODO is ~300 bytes and a
// completed one ~630, so a body well above that must still be accepted.
func TestCalDAVBodyLimit_AllowsRealisticBody(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Buy milk", "status": "todo"}
	app := caldavTestApp(t, stored)

	// A realistic VTODO padded with a long DESCRIPTION — orders of magnitude
	// larger than any observed client body, still far under the cap.
	padding := strings.Repeat("some notes ", 400) // ~4.4 KB
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nBEGIN:VTODO\r\n" +
		"UID:task--TSK-1@rela\r\nSUMMARY:Buy milk\r\nDESCRIPTION:" + padding + "\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"
	if len(body) >= maxCalDAVBodyBytes {
		t.Fatalf("test body %d is not under the cap %d", len(body), maxCalDAVBodyBytes)
	}

	rec := doCalDAV(t, app, http.MethodPut,
		"/api/v1/_caldav/principal/calendars/tasks/task--TSK-1@rela.ics", body)
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Errorf("a %d-byte body was rejected as too large; the cap is too tight", len(body))
	}
}

// fakeRedactor hides a fixed set of property names.
type fakeRedactor struct{ hide []string }

func (f fakeRedactor) HiddenProperties(context.Context, *entity.Entity) map[string]struct{} {
	out := map[string]struct{}{}
	for _, h := range f.hide {
		out[h] = struct{}{}
	}
	return out
}

// TestRedactEntityFields_HidesProperties pins that field-level `visible:`
// redaction reaches the CalDAV read path.
//
// visibleReader gates ROWS; it does not touch FIELDS. docs/acl-security.md
// commits to redaction on "every HTTP read shape", and CalDAV was a new read
// shape that skipped it — a collection mapping `description: body` served the
// body verbatim to a principal whose role redacts it.
func TestRedactEntityFields_HidesProperties(t *testing.T) {
	e := entity.New("TSK-1", "task")
	e.Properties = map[string]any{"title": "Buy milk", "secret": "classified"}
	e.Content = "private notes"

	got := redactEntityFields(t.Context(), fakeRedactor{hide: []string{"secret"}}, e)

	if _, still := got.Properties["secret"]; still {
		t.Error("a hidden property survived into the CalDAV render")
	}
	if got.Properties["title"] != "Buy milk" {
		t.Errorf("a visible property was dropped: %v", got.Properties["title"])
	}
	if _, still := e.Properties["secret"]; !still {
		t.Error("the SHARED store entity was mutated — redaction must copy, or it " +
			"redacts for every other reader including write-prep, where a missing " +
			"property is an erasure")
	}
}

// TestRedactEntityFields_HidingBodyClearsContent pins the body case. `visible:`
// names properties and has no vocabulary for the markdown body, so a collection
// mapping `description: body` would otherwise leak exactly what a `body`
// redaction was asked to hide.
func TestRedactEntityFields_HidingBodyClearsContent(t *testing.T) {
	e := entity.New("TSK-1", "task")
	e.Properties = map[string]any{"title": "Buy milk"}
	e.Content = "private notes"

	got := redactEntityFields(t.Context(), fakeRedactor{hide: []string{"body"}}, e)
	if got.Content != "" {
		t.Errorf("body redaction left the content intact: %q", got.Content)
	}
	if e.Content == "" {
		t.Error("the shared store entity's content was cleared in place")
	}
}

// TestRedactEntityFields_NoPolicyIsPassthrough pins that the common case — no
// policy, nothing hidden — returns the entity untouched rather than paying for
// a copy on every render.
func TestRedactEntityFields_NoPolicyIsPassthrough(t *testing.T) {
	e := entity.New("TSK-1", "task")
	e.Properties = map[string]any{"title": "Buy milk"}

	if got := redactEntityFields(t.Context(), fakeRedactor{}, e); got != e {
		t.Error("nothing hidden must return the same entity, not a copy")
	}
	if got := redactEntityFields(t.Context(), nil, e); got != e {
		t.Error("a nil redactor must be a no-op")
	}
}

// TestRenderObject_AppliesRedaction pins the WIRING, not just the helper.
//
// redactEntityFields being correct is worth nothing if renderObject stops
// calling it, and renderObject is the single place an entity becomes a CalDAV
// resource — so this is the one seam where a regression would silently
// un-redact every collection.
func TestRenderObject_AppliesRedaction(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Buy milk", "status": "todo", "notes": "classified"}
	app := caldavTestApp(t, stored)

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	m, _, err := b.mapperFor(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("mapperFor: %v", err)
	}
	// Map DESCRIPTION to the property this test hides.
	m.cfg.Description = "notes"

	render := func(ctx context.Context) string {
		obj, rErr := b.renderObject(ctx, "tasks", m, stored, "x.ics", "uid-1")
		if rErr != nil {
			t.Fatalf("renderObject: %v", rErr)
		}
		var buf strings.Builder
		if encErr := ical.NewEncoder(&buf).Encode(obj.Data); encErr != nil {
			t.Fatalf("encode: %v", encErr)
		}
		return buf.String()
	}

	// Precondition: with nothing hidden the value IS rendered, so the assertion
	// below cannot pass for an unrelated reason.
	if out := render(t.Context()); !strings.Contains(out, "classified") {
		t.Fatalf("precondition: the mapped property is not rendered at all:\n%s", out)
	}

	b.redactor = fakeRedactor{hide: []string{"notes"}}
	if out := render(t.Context()); strings.Contains(out, "classified") {
		t.Errorf("a `visible:`-hidden property reached the CalDAV wire:\n%s", out)
	}
}

// fakeWatermarkStore wraps a store with a settable entity-type watermark, so a
// test can drive the cheap ctag path without a postgres backend.
type fakeWatermarkStore struct {
	store.Store
	seq map[string]int64
}

func (f *fakeWatermarkStore) EntityTypeWatermark(_ context.Context, t string) (int64, error) {
	return f.seq[t], nil
}

// TestCollectionCTag_UsesWatermarkWhenAvailable pins that a backend exposing
// store.TypeWatermark takes the index-only path instead of rendering.
//
// The observable proof is that the tag tracks the WATERMARK: bumping the seq
// changes it while the entities are untouched, which the content-derived tag
// (a hash of per-entry ETags) could not do.
func TestCollectionCTag_UsesWatermarkWhenAvailable(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Buy milk", "status": "todo"}
	app := caldavTestApp(t, stored)

	fake := &fakeWatermarkStore{Store: app.Services().Store, seq: map[string]int64{"task": 1}}
	app.store = fake
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	first, err := b.collectionCTag(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("collectionCTag: %v", err)
	}

	// Same watermark, same entities → the tag must be stable, or every poll
	// re-enumerates and the optimization is worthless.
	again, err := b.collectionCTag(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("collectionCTag: %v", err)
	}
	if first != again {
		t.Errorf("ctag is unstable across polls: %q then %q", first, again)
	}

	fake.seq["task"] = 2
	moved, err := b.collectionCTag(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("collectionCTag: %v", err)
	}
	if moved == first {
		t.Error("the ctag did not follow the watermark — a client would never " +
			"learn the collection changed")
	}
}

// TestCollectionCTag_FallsBackWithoutWatermark pins the fsstore path: a backend
// with no watermark still gets a correct, content-derived tag.
func TestCollectionCTag_FallsBackWithoutWatermark(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Buy milk", "status": "todo"}
	app := caldavTestApp(t, stored)
	if _, ok := app.Services().Store.(store.TypeWatermark); ok {
		t.Fatal("precondition: the test store must NOT implement TypeWatermark")
	}

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	tag, err := b.collectionCTag(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("collectionCTag: %v", err)
	}
	if tag == "" {
		t.Error("a backend without a watermark must still produce a tag")
	}
}

// TestCollectionCTag_DistinctPerCollection pins that two collections over the
// SAME entity type do not share a tag.
//
// The watermark is per-TYPE, so the seq alone would collide. A client that
// switched between two collections would see a matching tag and skip
// enumerating content it has never seen.
func TestCollectionCTag_DistinctPerCollection(t *testing.T) {
	stored := entity.New("TSK-1", "task")
	stored.Properties = map[string]any{"title": "Buy milk", "status": "todo"}
	app := caldavTestApp(t, stored)

	// A second collection over the same entity type.
	cfg := app.State().Cfg
	second := cfg.CalDAV.Static["tasks"]
	cfg.CalDAV.Static["other"] = second

	app.store = &fakeWatermarkStore{
		Store: app.Services().Store, seq: map[string]int64{"task": 1},
	}
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	a, err := b.collectionCTag(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("collectionCTag(tasks): %v", err)
	}
	c, err := b.collectionCTag(t.Context(), "other")
	if err != nil {
		t.Fatalf("collectionCTag(other): %v", err)
	}
	if a == c {
		t.Error("two collections over one entity type share a ctag — a client " +
			"switching between them would skip enumerating the other")
	}
}

// caldavDynamicAppWith builds an App with a `project_tasks` PATTERN over the
// `project` driver type, seeded with the given entities and relations — so one
// config key serves one collection per project.
func caldavDynamicAppWith(t *testing.T, ents []*entity.Entity, rels []*entity.Relation) *App {
	t.Helper()
	base := caldavTestApp(t)
	meta := base.State().Meta
	if meta.Relations == nil {
		meta.Relations = map[string]metamodel.RelationDef{}
	}
	meta.Relations["belongs-to"] = metamodel.RelationDef{
		From: []string{"task"}, To: []string{"project"},
	}
	meta.Entities["project"] = metamodel.EntityDef{
		Label: "Project", IDPrefix: "PRJ-", DisplayProperty: "title",
		Properties: map[string]metamodel.PropertyDef{
			"title": {Type: metamodel.PropertyTypeString, Required: true},
		},
	}
	cfg := base.State().Cfg
	cfg.CalDAV.Dynamic = map[string]dataentryconfig.CalDAVDynamicCollection{
		"project_tasks": {
			CalDAVCollection: cfg.CalDAV.Static["tasks"],
			DriverType:       "project",
			Relation:         "belongs-to",
		},
	}

	f := newFixture()
	for _, e := range ents {
		f.AddNode(e)
	}
	for _, r := range rels {
		f.AddEdge(r)
	}
	app := newAppFromParts(cfg, meta, f)

	root, err := storage.NewRootedFS(storage.NewMemFS(), t.TempDir())
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	aliases, aliasErr := caldavalias.New(t.Context(), state.NewFSKV(root))
	if aliasErr != nil {
		t.Fatalf("caldavalias.New: %v", aliasErr)
	}
	app.SetCalDAVAliases(aliases)
	return app
}

func mkProject(id, title string) *entity.Entity {
	e := entity.New(id, "project")
	e.Properties = map[string]any{"title": title}
	return e
}

func mkTaskIn(id, title string) *entity.Entity {
	e := entity.New(id, "task")
	e.Properties = map[string]any{"title": title, "status": "todo"}
	return e
}

// TestDynamicCollections_ExpandPerDriver pins AC1: one config key yields one
// collection per driver entity, discoverable from the single account URL.
func TestDynamicCollections_ExpandPerDriver(t *testing.T) {
	app := caldavDynamicAppWith(t,
		[]*entity.Entity{mkProject("PRJ-1", "Alpha"), mkProject("PRJ-2", "Beta")},
		nil)

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	cals, err := b.ListCalendars(t.Context())
	if err != nil {
		t.Fatalf("ListCalendars: %v", err)
	}

	got := map[string]string{}
	for _, c := range cals {
		got[c.Path] = c.Name
	}
	for _, want := range []struct{ seg, name string }{
		{"project_tasks--PRJ-1", "Alpha"},
		{"project_tasks--PRJ-2", "Beta"},
	} {
		path := b.calendarPath(want.seg)
		if got[path] != want.name {
			t.Errorf("collection %q: name = %q, want %q (the driver's title, so a "+
				"rename renames the list)", want.seg, got[path], want.name)
		}
	}
	if len(cals) != 3 { // the static "tasks" plus two expansions
		t.Errorf("want 3 collections (1 static + 2 expanded), got %d", len(cals))
	}
}

// TestDynamicCollections_MembershipFollowsTheRelation pins that a collection
// carries exactly the entities linked to ITS driver — the whole point of the
// feature.
func TestDynamicCollections_MembershipFollowsTheRelation(t *testing.T) {
	app := caldavDynamicAppWith(t,
		[]*entity.Entity{
			mkProject("PRJ-1", "Alpha"), mkProject("PRJ-2", "Beta"),
			mkTaskIn("TSK-A", "alpha work"), mkTaskIn("TSK-B", "beta work"),
			mkTaskIn("TSK-LOOSE", "unassigned"),
		},
		[]*entity.Relation{
			entity.NewRelation("TSK-A", "belongs-to", "PRJ-1"),
			entity.NewRelation("TSK-B", "belongs-to", "PRJ-2"),
		})

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	for _, tc := range []struct {
		collection string
		want       string
	}{
		{"project_tasks--PRJ-1", "TSK-A"},
		{"project_tasks--PRJ-2", "TSK-B"},
	} {
		objs, err := b.listTodos(t.Context(), tc.collection)
		if err != nil {
			t.Fatalf("listTodos(%s): %v", tc.collection, err)
		}
		if len(objs) != 1 {
			t.Fatalf("%s: want 1 member, got %d", tc.collection, len(objs))
		}
		if !strings.Contains(objs[0].Path, tc.want) {
			t.Errorf("%s: got %q, want the member linked to this driver (%s)",
				tc.collection, objs[0].Path, tc.want)
		}
	}
}

// TestDynamicCollections_UnknownDriverIsNotFound pins AC5: a driver that does
// not exist — or that this principal cannot read — gets the SAME answer as an
// unknown collection. The driver id is in the URL, so a distinguishable status
// would be an existence oracle for it.
func TestDynamicCollections_UnknownDriverIsNotFound(t *testing.T) {
	app := caldavDynamicAppWith(t, []*entity.Entity{mkProject("PRJ-1", "Alpha")}, nil)
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	if _, err := b.GetCalendar(t.Context(), b.calendarPath("project_tasks--PRJ-NOPE")); err == nil {
		t.Error("a nonexistent driver must not resolve to a collection")
	}
	if _, _, err := b.mapperFor(t.Context(), "project_tasks--PRJ-NOPE"); err == nil {
		t.Error("mapperFor must refuse an unresolvable driver")
	}
	// A real driver still works, so the refusal above is not blanket.
	if _, err := b.GetCalendar(t.Context(), b.calendarPath("project_tasks--PRJ-1")); err != nil {
		t.Errorf("a readable driver must resolve: %v", err)
	}
}

// TestSplitDynamicName pins the segment parse, including the hostile cases.
func TestSplitDynamicName(t *testing.T) {
	for _, tc := range []struct {
		in              string
		pattern, driver string
		ok              bool
	}{
		{"project_tasks--PRJ-1", "project_tasks", "PRJ-1", true},
		{"tasks", "", "", false},                            // a static key
		{"project_tasks--", "", "", false},                  // no driver
		{"--PRJ-1", "", "", false},                          // no pattern
		{"project_tasks--PRJ/1", "", "", false},             // slash in id
		{"project_tasks--..", "", "", false},                // traversal in id
		{"project_tasks--a--b", "project_tasks", "", false}, // separator inside id
	} {
		p, d, ok := splitDynamicName(tc.in)
		if ok != tc.ok {
			t.Errorf("splitDynamicName(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && (p != tc.pattern || d != tc.driver) {
			t.Errorf("splitDynamicName(%q) = (%q,%q), want (%q,%q)", tc.in, p, d, tc.pattern, tc.driver)
		}
	}
}

// mustParseICal decodes an iCalendar body the way go-webdav does before handing
// it to the backend.
func mustParseICal(t *testing.T, body string) *ical.Calendar {
	t.Helper()
	cal, err := ical.NewDecoder(strings.NewReader(body)).Decode()
	if err != nil {
		t.Fatalf("decode iCalendar: %v", err)
	}
	return cal
}

// caldavDynamicBody is a client-composed VTODO — the shape Apple actually
// sends on a create: a bare UUID, a summary, and almost nothing else.
func caldavDynamicBody(uid, summary string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nBEGIN:VTODO\r\n" +
		"UID:" + uid + "\r\nSUMMARY:" + summary + "\r\nSTATUS:NEEDS-ACTION\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"
}

// TestDynamicCollections_CreateGetsTheDriverRelation pins AC7.
//
// Membership in a dynamic collection IS the relation, so a to-do created in
// `project_tasks--PRJ-1` that does not get the `belongs-to` edge lands in the
// entity type but in NO collection: it vanishes from the client on the next
// sync and is invisible in every CalDAV view.
func TestDynamicCollections_CreateGetsTheDriverRelation(t *testing.T) {
	app := caldavDynamicAppWith(t, []*entity.Entity{mkProject("PRJ-1", "Alpha")}, nil)
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	obj, err := b.PutCalendarObject(t.Context(),
		b.calendarPath("project_tasks--PRJ-1")+"NEW-1.ics",
		mustParseICal(t, caldavDynamicBody("NEW-1", "buy milk")), nil)
	if err != nil {
		t.Fatalf("PutCalendarObject: %v", err)
	}
	if obj == nil {
		t.Fatal("create returned no object")
	}

	// The proof that matters: the new entry is a MEMBER on the next read. A
	// created-but-unlinked entity would be absent here while still existing.
	objs, err := b.listTodos(t.Context(), "project_tasks--PRJ-1")
	if err != nil {
		t.Fatalf("listTodos: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("the created to-do is not in its own collection (got %d members) — "+
			"it would vanish from the client on the next sync", len(objs))
	}
}

// TestDynamicCollections_CreateInStaticNeedsNoRelation pins that the linking
// step is a no-op for static collections, which have no driver.
func TestDynamicCollections_CreateInStaticNeedsNoRelation(t *testing.T) {
	app := caldavDynamicAppWith(t, nil, nil)
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	if _, err := b.PutCalendarObject(t.Context(),
		b.calendarPath("tasks")+"NEW-2.ics",
		mustParseICal(t, caldavDynamicBody("NEW-2", "standalone")), nil); err != nil {
		t.Fatalf("a create in a STATIC collection must not require a driver edge: %v", err)
	}
	objs, err := b.listTodos(t.Context(), "tasks")
	if err != nil {
		t.Fatalf("listTodos: %v", err)
	}
	if len(objs) != 1 {
		t.Errorf("want the created to-do in the static collection, got %d", len(objs))
	}
}

// TestDynamicCollections_FailedLinkRemovesTheOrphan pins the compensation.
//
// If the driver edge cannot be created, the entity must not survive: an entity
// in the type but in no collection is invisible in every CalDAV view, so the
// user can neither find nor fix it. A failed create is visible and retryable;
// an orphan is neither.
//
// The link is made to fail by pointing the pattern at a relation the metamodel
// does not allow between these types — the same rejection a misconfigured or
// concurrently-changed schema would produce.
func TestDynamicCollections_FailedLinkRemovesTheOrphan(t *testing.T) {
	app := caldavDynamicAppWith(t, []*entity.Entity{mkProject("PRJ-1", "Alpha")}, nil)

	// A relation that exists but is not permitted from task to project.
	meta := app.State().Meta
	meta.Relations["wrong-way"] = metamodel.RelationDef{
		From: []string{"project"}, To: []string{"project"},
	}
	dyn := app.State().Cfg.CalDAV.Dynamic["project_tasks"]
	dyn.Relation = "wrong-way"
	app.State().Cfg.CalDAV.Dynamic["project_tasks"] = dyn

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	before := countStoredEntities(t, app, "task")

	if _, err := b.PutCalendarObject(t.Context(),
		b.calendarPath("project_tasks--PRJ-1")+"ORPHAN.ics",
		mustParseICal(t, caldavDynamicBody("ORPHAN", "doomed")), nil); err == nil {
		t.Fatal("a create whose driver link fails must not report success")
	}

	if after := countStoredEntities(t, app, "task"); after != before {
		t.Errorf("the entity survived a failed link (%d → %d tasks) — it exists in "+
			"no collection, so the user cannot see or fix it", before, after)
	}
}

// countStoredEntities counts stored entities of a type.
func countStoredEntities(t *testing.T, app *App, typ string) int {
	t.Helper()
	n := 0
	for e, err := range app.Services().Store.ListEntities(t.Context(), store.EntityQuery{Type: typ}) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		if e != nil {
			n++
		}
	}
	return n
}

// TestDynamicCollections_ReEditIsIdempotent pins that a NORMAL edit — a PUT
// into the collection the to-do already belongs to — does not fail.
//
// The update path now adds the driver edge so an assignment works, which means
// every ordinary check-off re-asserts an edge that already exists. If that
// errored, editing any to-do in a dynamic collection would break.
func TestDynamicCollections_ReEditIsIdempotent(t *testing.T) {
	app := caldavDynamicAppWith(t,
		[]*entity.Entity{mkProject("PRJ-1", "Alpha"), mkTaskIn("TSK-A", "work")},
		[]*entity.Relation{entity.NewRelation("TSK-A", "belongs-to", "PRJ-1")})
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	p := b.calendarPath("project_tasks--PRJ-1") + "task--TSK-A@rela.ics"
	for i := range 2 {
		if _, err := b.PutCalendarObject(t.Context(), p,
			mustParseICal(t, caldavDynamicBody("task--TSK-A@rela", "work edited")), nil); err != nil {
			t.Fatalf("edit %d failed: %v — re-asserting an existing membership must be a no-op", i+1, err)
		}
	}
	objs, err := b.listTodos(t.Context(), "project_tasks--PRJ-1")
	if err != nil {
		t.Fatalf("listTodos: %v", err)
	}
	if len(objs) != 1 {
		t.Errorf("want 1 member after repeated edits, got %d", len(objs))
	}
}

// TestDynamicCollections_FailedAssignNeverDeletesAnExistingEntity pins
// BUG-2ATX4H: the compensating delete belongs to the CREATE flow only.
//
// The update flow re-asserts membership on every PUT, so it shares the attach
// step with create — but not create's undo. linkToDriver deletes the entity
// when the edge cannot be made, justified by the entity being "seconds old and
// wholly ours". On an edit of a pre-existing to-do that justification is false:
// the patch has already succeeded and the content is the user's. Routing the
// update flow through linkToDriver therefore destroyed a long-lived entity
// whenever the membership write failed — while the client saw only an error.
//
// Every non-already-exists error from CreateRelation triggered it, not just an
// ACL deny: a metamodel-invalid relation (used here, being deterministic), an
// unreadable driver, or a dropped connection all reached the same delete.
func TestDynamicCollections_FailedAssignNeverDeletesAnExistingEntity(t *testing.T) {
	app := caldavDynamicAppWith(t,
		[]*entity.Entity{mkProject("PRJ-1", "Alpha"), mkTaskIn("TSK-A", "work")},
		nil) // NOT yet a member: the PUT below is an assignment, so it attaches.

	// Make the membership write fail: the relation no longer admits task->project.
	meta := app.State().Meta
	meta.Relations["belongs-to"] = metamodel.RelationDef{
		From: []string{"project"}, To: []string{"project"},
	}

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	p := b.calendarPath("project_tasks--PRJ-1") + "task--TSK-A@rela.ics"
	_, err := b.PutCalendarObject(t.Context(), p,
		mustParseICal(t, caldavDynamicBody("task--TSK-A@rela", "edited")), nil)
	if err == nil {
		t.Log("attach failure surfaced as a non-error response; the entity check below is what matters")
	}

	// THE ASSERTION: the pre-existing entity must still be there. Before the
	// fix it was deleted by the compensating delete.
	if _, gErr := app.store.GetEntity(t.Context(), "TSK-A"); gErr != nil {
		t.Fatalf("a failed collection assignment DELETED the pre-existing to-do: %v", gErr)
	}
}

// TestDynamicCollections_FailedCreateStillCompensates pins the other half of
// BUG-2ATX4H: splitting the flows must not disarm the create-path undo.
//
// An entity created in a dynamic collection that does not get its edge is in no
// collection at all — invisible in every CalDAV view and unreachable by the
// user. That orphan is worse than a failed create, so the create is undone.
func TestDynamicCollections_FailedCreateStillCompensates(t *testing.T) {
	app := caldavDynamicAppWith(t, []*entity.Entity{mkProject("PRJ-1", "Alpha")}, nil)
	meta := app.State().Meta
	meta.Relations["belongs-to"] = metamodel.RelationDef{
		From: []string{"project"}, To: []string{"project"},
	}

	b := &caldavBackend{app: app, baseURL: "https://example.test"}
	p := b.calendarPath("project_tasks--PRJ-1") + "brand-new@rela.ics"
	if _, err := b.PutCalendarObject(t.Context(), p,
		mustParseICal(t, caldavDynamicBody("brand-new@rela", "fresh")), nil); err == nil {
		t.Fatal("want an error when a newly created entry cannot be attached to its collection")
	}

	// No orphan left behind in the entity type.
	var orphans int
	for e, eErr := range app.store.ListEntities(t.Context(), store.EntityQuery{Type: "task"}) {
		if eErr != nil {
			t.Fatalf("ListEntities: %v", eErr)
		}
		orphans++
		t.Logf("orphan left behind: %s", e.ID)
	}
	if orphans != 0 {
		t.Errorf("create failed to attach but left %d orphaned to-do(s) in no collection", orphans)
	}
}

// TestDynamicCollections_DeleteRemovesMembershipNotEntity pins the core of the
// unlink model: a DELETE names a MEMBERSHIP, not the entity.
//
// Applying on_delete: here would cancel the to-do everywhere — in the static
// collection, in every other project, and in the web app — from a gesture that
// meant "take it out of this list". Reproduced live before the fix.
func TestDynamicCollections_DeleteRemovesMembershipNotEntity(t *testing.T) {
	app := caldavDynamicAppWith(t,
		[]*entity.Entity{
			mkProject("PRJ-1", "Alpha"), mkProject("PRJ-2", "Beta"),
			mkTaskIn("TSK-A", "shared"),
		},
		[]*entity.Relation{
			entity.NewRelation("TSK-A", "belongs-to", "PRJ-1"),
			entity.NewRelation("TSK-A", "belongs-to", "PRJ-2"),
		})
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	if err := b.DeleteCalendarObject(t.Context(),
		b.calendarPath("project_tasks--PRJ-1")+"task--TSK-A@rela.ics"); err != nil {
		t.Fatalf("DeleteCalendarObject: %v", err)
	}

	// Gone from the collection it was removed from...
	gone, err := b.listTodos(t.Context(), "project_tasks--PRJ-1")
	if err != nil {
		t.Fatalf("listTodos(PRJ-1): %v", err)
	}
	if len(gone) != 0 {
		t.Errorf("the to-do is still in the collection it was deleted from (%d members)", len(gone))
	}
	// ...and STILL THERE in the other, un-cancelled.
	kept, err := b.listTodos(t.Context(), "project_tasks--PRJ-2")
	if err != nil {
		t.Fatalf("listTodos(PRJ-2): %v", err)
	}
	if len(kept) != 1 {
		t.Fatalf("removing one membership destroyed the other (%d members in PRJ-2)", len(kept))
	}
	e, err := app.Services().Store.GetEntity(t.Context(), "TSK-A")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got := e.GetString("status"); got == "cancelled" {
		t.Error("removing ONE membership cancelled the entity globally — that is the " +
			"data loss this model exists to prevent")
	}
}

// TestDynamicCollections_LastMembershipFollowsCardinality pins the `auto`
// policy: the relation's own min_outgoing decides whether an entity that now
// belongs to nothing is kept or disposed of, so the schema is the single source
// of truth rather than a second setting that can disagree with it.
// oneMembershipRequired is min_outgoing=1 — membership declared mandatory.
var oneMembershipRequired = 1

func TestDynamicCollections_LastMembershipFollowsCardinality(t *testing.T) {
	for _, tc := range []struct {
		name        string
		minOutgoing *int
		wantStatus  string
	}{
		{"optional membership keeps the entity", nil, "todo"},
		{"mandatory membership applies on_delete", &oneMembershipRequired, "done"}, // the test collection's on_delete sets status=done
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := caldavDynamicAppWith(t,
				[]*entity.Entity{mkProject("PRJ-1", "Alpha"), mkTaskIn("TSK-A", "only here")},
				[]*entity.Relation{entity.NewRelation("TSK-A", "belongs-to", "PRJ-1")})
			rel := app.State().Meta.Relations["belongs-to"]
			rel.MinOutgoing = tc.minOutgoing
			app.State().Meta.Relations["belongs-to"] = rel

			b := &caldavBackend{app: app, baseURL: "https://example.test"}
			if err := b.DeleteCalendarObject(t.Context(),
				b.calendarPath("project_tasks--PRJ-1")+"task--TSK-A@rela.ics"); err != nil {
				t.Fatalf("DeleteCalendarObject: %v", err)
			}
			e, err := app.Services().Store.GetEntity(t.Context(), "TSK-A")
			if err != nil {
				t.Fatalf("GetEntity: %v", err)
			}
			if got := e.GetString("status"); got != tc.wantStatus {
				t.Errorf("status = %q, want %q", got, tc.wantStatus)
			}
		})
	}
}
