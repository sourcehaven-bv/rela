package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	synctypes "github.com/Sourcehaven-BV/rela/internal/sync"
)

// The sync record read/write channel (/api/sync/entities|relations) was retired
// in TKT-8P1TM7 — the sync client now reads and writes through the authorized
// /api/v1 API. Only the change feed (/api/sync/manifest) remains sync-specific,
// so these tests cover the manifest handler and the /api/sync/ CSRF exemption.

// manifestStore wraps a store.Store and adds a canned ManifestSince, so the
// manifest HANDLER (serialization + cursor + ACL filtering) can be tested
// without a Postgres backend (only ManifestSince itself is pg-specific; it has
// its own DB-gated tests in pgstore).
type manifestStore struct {
	store.Store
	entries []synctypes.ManifestEntry
}

func (m manifestStore) ManifestSince(_ context.Context, cursor int64) ([]synctypes.ManifestEntry, error) {
	var out []synctypes.ManifestEntry
	for _, e := range m.entries {
		if e.Seq > cursor {
			out = append(out, e)
		}
	}
	return out, nil
}

// manifestRequest issues a GET through the full router with NO Origin header —
// modeling a non-browser sync client — to exercise the /api/sync/ same-origin
// exemption alongside the manifest handler.
func manifestRequest(t *testing.T, app *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	w := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(w, r)
	return w
}

// TestSync_SameOriginExemption: with the security middleware ACTIVE, a bare
// non-browser request (no Origin, no Cookie, no Sec-Fetch-Site) to a path the
// sync CLI uses — /api/sync/manifest AND the /api/v1 data + schema routes
// (TKT-8P1TM7) — is NOT rejected as origin_missing. A browser-shaped v1 surface
// (/api/v1/_search) and a browser-signaled request stay same-origin gated.
func TestSync_SameOriginExemption(t *testing.T) {
	app := newHandlerTestApp(t)
	// Wire the security middleware (newHandlerTestApp leaves it nil). Loopback
	// bind so 127.0.0.1:8080 is an allowed Host.
	sec, err := newSecurity(SecurityConfig{BindAddress: "127.0.0.1:8080"})
	if err != nil {
		t.Fatalf("newSecurity: %v", err)
	}
	app.security = sec
	handler := app.NewRouter()

	// A bare non-browser request: correct Host, but NO Origin/Cookie/Sec-Fetch —
	// the exact shape the sync CLI produces (setup can add browser signals).
	bare := func(method, path string, setup func(*http.Request)) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, http.NoBody)
		r.Host = "127.0.0.1:8080"
		if setup != nil {
			setup(r)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	// The sync CLI's paths — /api/sync/manifest AND the /api/v1 data + schema
	// routes — must NOT be same-origin-rejected for a bare non-browser request.
	// (They may 404/501/etc. from the handler; the point is NOT a 403.)
	for _, path := range []string{
		"/api/sync/manifest",
		"/api/v1/_schema",
		"/api/v1/tickets/TKT-001",
		"/api/v1/tickets",
	} {
		if w := bare(http.MethodGet, path, nil); w.Code == http.StatusForbidden {
			t.Errorf("bare non-browser GET %s was 403 — sync CSRF exemption not working: %s", path, w.Body.String())
		}
	}

	// A v1 WRITE (PATCH) is likewise reachable for the bare CLI shape — the sync
	// push path must not be same-origin-blocked.
	if w := bare(http.MethodPatch, "/api/v1/tickets/TKT-001", func(r *http.Request) {
		r.Header.Set("Content-Type", "application/json")
	}); w.Code == http.StatusForbidden {
		t.Errorf("bare non-browser PATCH /api/v1/tickets/TKT-001 was 403 — sync push blocked: %s", w.Body.String())
	}

	// The exemption is SCOPED: an underscore v1 sub-surface (a browser/SPA route,
	// not a sync route) with no Origin stays same-origin-gated.
	if w := bare(http.MethodGet, "/api/v1/_search", nil); w.Code != http.StatusForbidden {
		t.Errorf("/api/v1/_search no-Origin: got %d, want 403 (must NOT be sync-exempt)", w.Code)
	}

	// The exemption is CONDITIONED on the provably-non-browser shape: the SAME v1
	// data path carrying a browser signal (Sec-Fetch-Site, Cookie, or Origin) is
	// still same-origin gated — a browser fetch() cannot ride the exemption.
	for name, sig := range map[string]func(*http.Request){
		"Sec-Fetch-Site": func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"Cookie":         func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "session", Value: "x"}) },
		"Origin":         func(r *http.Request) { r.Header.Set("Origin", "https://evil.com") },
	} {
		if w := bare(http.MethodGet, "/api/v1/tickets/TKT-001", sig); w.Code != http.StatusForbidden {
			t.Errorf("v1 data path with %s must be same-origin gated: got %d, want 403", name, w.Code)
		}
	}
}

// TestSync_CSRFExemptionRequiresNoCookie is the C1 security regression: the
// /api/sync/ same-origin exemption must NOT apply to a browser-credentialed
// request (one carrying a Cookie, or a cross-origin Origin), or a malicious page
// could ride a victim's proxy session. Such a request must be rejected like any
// other cross-origin write.
//
// This heuristic exists because a header-trust proxy (oauth2-proxy, Authelia,
// Vouch, …) normalizes both its cookie-session browser and the Bearer-token CLI
// into the same X-Forwarded-User, so the app cannot tell them apart from what the
// proxy forwards — see the nonBrowserExemptPrefixes doc for why it's load-bearing
// and when it retires (FEAT-ESLP / proxy Cookie-stripping).
func TestSync_CSRFExemptionRequiresNoCookie(t *testing.T) {
	app := newHandlerTestApp(t)
	sec, err := newSecurity(SecurityConfig{BindAddress: "127.0.0.1:8080"})
	if err != nil {
		t.Fatalf("newSecurity: %v", err)
	}
	app.security = sec
	handler := app.NewRouter()

	doReq := func(setup func(*http.Request)) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/sync/manifest", http.NoBody)
		r.Host = "127.0.0.1:8080"
		setup(r)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	// A request carrying a Cookie (a browser with ambient proxy session) must be
	// rejected by same-origin despite the /api/sync/ path.
	withCookie := doReq(func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "session", Value: "victim-proxy-session"})
	})
	if withCookie.Code != http.StatusForbidden {
		t.Errorf("cookie-bearing /api/sync request: got %d, want 403 (CSRF must not be exempt)", withCookie.Code)
	}

	// A request with a cross-origin Origin (a browser fetch) must also be
	// rejected — evil.com is not an allowed origin.
	withOrigin := doReq(func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.com")
	})
	if withOrigin.Code != http.StatusForbidden {
		t.Errorf("cross-origin /api/sync request: got %d, want 403", withOrigin.Code)
	}

	// A request carrying Sec-Fetch-Site (a real browser — JS cannot forge it)
	// must NOT be exempt, even with no cookie.
	crossSiteFetch := doReq(func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
	})
	if crossSiteFetch.Code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site:cross-site /api/sync request: got %d, want 403 (browser must not be exempt)", crossSiteFetch.Code)
	}
	sameOriginFetch := doReq(func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if sameOriginFetch.Code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site present must defeat the exemption: got %d, want 403", sameOriginFetch.Code)
	}
}

// TestSync_ManifestUnsupportedOnNonPostgres: the manifest degrades to 501 on a
// backend that does not implement the manifest source.
func TestSync_ManifestUnsupportedOnNonPostgres(t *testing.T) {
	app := newHandlerTestApp(t)
	if app.sync.manifest != nil {
		t.Skip("test backend unexpectedly supports the manifest")
	}
	w := manifestRequest(t, app, "/api/sync/manifest")
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("manifest on non-pg backend: got %d, want 501 (%s)", w.Code, w.Body.String())
	}
}

// TestSync_ManifestSerialization: the manifest handler serializes entries (live
// + tombstone) to the wire shape and returns the highest seq as the next cursor.
func TestSync_ManifestSerialization(t *testing.T) {
	app := newHandlerTestApp(t)
	app.store = manifestStore{Store: app.store, entries: []synctypes.ManifestEntry{
		{Kind: "e", IDA: "TKT-1", Typ: "ticket", Deleted: false, Seq: 5},
		{Kind: "e", IDA: "TKT-2", Typ: "ticket", Deleted: true, Seq: 6}, // tombstone
		{Kind: "r", IDA: "TKT-1", IDB: "belongs_to", IDC: "CMP-1", Deleted: false, Seq: 7},
	}}
	rebindSyncHandler(app) // re-resolve the manifest capability against the swapped store

	w := manifestRequest(t, app, "/api/sync/manifest?cursor=4")
	if w.Code != http.StatusOK {
		t.Fatalf("manifest: %d %s", w.Code, w.Body.String())
	}
	var resp syncManifestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Changes) != 3 {
		t.Fatalf("changes = %d, want 3", len(resp.Changes))
	}
	if resp.Cursor != "7" {
		t.Errorf("cursor = %q, want 7 (highest seq)", resp.Cursor)
	}
	// Tombstone flag + relation key shape.
	if !resp.Changes[1].Deleted {
		t.Error("second change should be a tombstone")
	}
	if resp.Changes[2].ID != "TKT-1/belongs_to/CMP-1" {
		t.Errorf("relation id = %q, want slash triple form", resp.Changes[2].ID)
	}

	// A cursor past everything returns no changes but keeps the cursor.
	w2 := manifestRequest(t, app, "/api/sync/manifest?cursor=99")
	var resp2 syncManifestResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if len(resp2.Changes) != 0 {
		t.Errorf("expected no changes past cursor, got %d", len(resp2.Changes))
	}
}

// TestSync_Manifest_ACLFiltered: the manifest drops rows the principal may not
// read (row-level gate), while the cursor still advances past them so the client
// does not re-poll the hidden tail forever.
func TestSync_Manifest_ACLFiltered(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "F1"}})
	seedEntity(app, &entity.Entity{ID: "CMP-1", Type: "component", Properties: map[string]any{"name": "Core"}})

	app.store = manifestStore{Store: app.store, entries: []synctypes.ManifestEntry{
		{Kind: "e", IDA: "TKT-1", Typ: "ticket", Deleted: false, Seq: 5},
		{Kind: "e", IDA: "FEAT-1", Typ: "feature", Deleted: false, Seq: 6}, // hidden from alice
		{Kind: "e", IDA: "TKT-2", Typ: "ticket", Deleted: true, Seq: 7},    // ticket tombstone — visible
		{Kind: "r", IDA: "FEAT-1", IDB: "needs", IDC: "CMP-1", Seq: 8},     // sourced on feature — hidden
		{Kind: "r", IDA: "TKT-1", IDB: "belongs_to", IDC: "CMP-1", Seq: 9}, // sourced on ticket — visible
	}}
	rebindSyncHandler(app) // re-resolve the manifest capability + gate reads against the swapped store

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/sync/manifest?cursor=4", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.sync.handleSyncManifest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("manifest: %d %s", rec.Code, rec.Body.String())
	}
	var resp syncManifestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Only the ticket entity, the ticket tombstone, and the ticket-sourced
	// relation survive — the feature entity and feature-sourced relation drop.
	gotIDs := map[string]bool{}
	for _, c := range resp.Changes {
		gotIDs[c.ID] = true
	}
	if len(resp.Changes) != 3 {
		t.Fatalf("changes = %d, want 3 (%v)", len(resp.Changes), gotIDs)
	}
	for _, want := range []string{"TKT-1", "TKT-2", "TKT-1/belongs_to/CMP-1"} {
		if !gotIDs[want] {
			t.Errorf("expected visible change %q missing; got %v", want, gotIDs)
		}
	}
	for _, hidden := range []string{"FEAT-1", "FEAT-1/needs/CMP-1"} {
		if gotIDs[hidden] {
			t.Errorf("hidden change %q leaked into manifest", hidden)
		}
	}
	// Cursor still advances to the highest seq (9), INCLUDING the dropped rows.
	if resp.Cursor != "9" {
		t.Errorf("cursor = %q, want 9 (highest seq over all rows, visible or not)", resp.Cursor)
	}
}
