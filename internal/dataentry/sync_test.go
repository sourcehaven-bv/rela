package dataentry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	synctypes "github.com/Sourcehaven-BV/rela/internal/sync"
)

// manifestStore wraps a store.Store and adds a canned ManifestSince, so the
// manifest HANDLER (serialization + cursor) can be tested without a Postgres
// backend (only ManifestSince itself is pg-specific; it has its own DB-gated
// tests in pgstore).
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

// syncRequest issues a request through the full router, optionally setting
// If-Match and a JSON body. It deliberately sends NO Origin header — modeling a
// non-browser sync client — to exercise the /api/sync/ same-origin exemption.
func syncRequest(t *testing.T, app *App, method, path, ifMatch string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, http.NoBody)
	}
	if ifMatch != "" {
		r.Header.Set("If-Match", ifMatch)
	}
	w := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(w, r)
	return w
}

// TestSync_SameOriginExemption: with the security middleware ACTIVE, a /api/sync/
// request with no Origin header is NOT rejected as origin_missing, whereas a
// /api/v1/ one IS. Proves the CSRF exemption (a non-browser client sends no
// Origin) while the Host check still applies to both.
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

	noOrigin := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		r.Host = "127.0.0.1:8080" // pass the Host check; deliberately set NO Origin
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	// Control: /api/v1 with no Origin is rejected by requireSameOrigin.
	ctl := noOrigin("/api/v1/tickets/TKT-001")
	if ctl.Code != http.StatusForbidden {
		t.Fatalf("control /api/v1 no-Origin: got %d, want 403 (sanity of the exemption test): %s", ctl.Code, ctl.Body.String())
	}

	// /api/sync with no Origin + no Cookie (a bare CLI) must NOT be 403 — the
	// exemption lets it through to the handler (200 for an existing entity).
	w := noOrigin("/api/sync/entities/TKT-001")
	if w.Code == http.StatusForbidden {
		t.Fatalf("/api/sync no-Origin no-Cookie was forbidden (%d) — exemption not working: %s", w.Code, w.Body.String())
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
		r := httptest.NewRequest(http.MethodPut, "/api/sync/entities/TKT-001", http.NoBody)
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
		t.Errorf("cookie-bearing /api/sync write: got %d, want 403 (CSRF must not be exempt)", withCookie.Code)
	}

	// A request with a cross-origin Origin (a browser fetch) must also be
	// rejected — evil.com is not an allowed origin.
	withOrigin := doReq(func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.com")
	})
	if withOrigin.Code != http.StatusForbidden {
		t.Errorf("cross-origin /api/sync write: got %d, want 403", withOrigin.Code)
	}

	// A request carrying Sec-Fetch-Site (a real browser — JS cannot forge it)
	// must NOT be exempt, even with no cookie. This is the JS-fetch vector: a
	// page's fetch() always carries Sec-Fetch-Site, so the cross-site value is
	// rejected by same-origin. The header is the primary, official signal.
	crossSiteFetch := doReq(func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
	})
	if crossSiteFetch.Code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site:cross-site /api/sync write: got %d, want 403 (browser must not be exempt)", crossSiteFetch.Code)
	}
	// Even a same-origin browser fetch carries Sec-Fetch-Site; it must fall
	// through to same-origin (which, with no allowed Origin set on it, is also
	// rejected) rather than be CSRF-exempted.
	sameOriginFetch := doReq(func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if sameOriginFetch.Code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site present must defeat the exemption: got %d, want 403", sameOriginFetch.Code)
	}
}

// TestSync_GetEntity returns the record body + an ETag equal to its hash.
func TestSync_GetEntity(t *testing.T) {
	app := newHandlerTestApp(t)
	w := syncRequest(t, app, http.MethodGet, "/api/sync/entities/TKT-001", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	if etag := w.Header().Get("ETag"); etag == "" {
		t.Fatal("missing ETag header")
	}
	var body syncEntityBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "TKT-001" {
		t.Fatalf("id = %q", body.ID)
	}
}

// TestSync_PushUpdate_HappyAnd412 covers: a push with the correct If-Match
// applies (200); a push with a stale If-Match conflicts (412).
func TestSync_PushUpdate_HappyAnd412(t *testing.T) {
	app := newHandlerTestApp(t)

	// Current hash of TKT-001 from the store.
	e, err := app.store.GetEntity(t.Context(), "TKT-001")
	if err != nil {
		t.Fatalf("seed get: %v", err)
	}
	base := canonical.HashEntity(*e)

	updated := syncEntityBody{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Updated via sync", "status": "open"},
	}

	// Happy path: correct If-Match → 200 + new ETag.
	w := syncRequest(t, app, http.MethodPut, "/api/sync/entities/TKT-001", base, updated)
	if w.Code != http.StatusOK {
		t.Fatalf("push happy: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("ETag") == base {
		t.Fatal("ETag did not change after update")
	}

	// Stale base: pushing again with the OLD hash → 412.
	w2 := syncRequest(t, app, http.MethodPut, "/api/sync/entities/TKT-001", base, updated)
	if w2.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale push: got %d, want 412 (%s)", w2.Code, w2.Body.String())
	}
}

// TestSync_PushCreate creates a new entity with no If-Match (first create) and
// preserves the supplied id even though it would be rejected by CreateEntity.
func TestSync_PushCreate(t *testing.T) {
	app := newHandlerTestApp(t)
	body := syncEntityBody{
		ID: "TKT-SYNC1", Type: "ticket",
		Properties: map[string]any{"title": "Synced", "status": "open"},
	}
	w := syncRequest(t, app, http.MethodPut, "/api/sync/entities/TKT-SYNC1", "", body)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	got, err := app.store.GetEntity(t.Context(), "TKT-SYNC1")
	if err != nil {
		t.Fatalf("created entity not found: %v", err)
	}
	if got.GetString("title") != "Synced" {
		t.Fatalf("title = %q", got.GetString("title"))
	}
}

// TestSync_PushCreate_NoIfMatchOnExisting: a push with NO If-Match against an
// EXISTING record is a 412 (the client must declare its base; a blind push
// could clobber).
func TestSync_PushCreate_NoIfMatchOnExisting(t *testing.T) {
	app := newHandlerTestApp(t)
	body := syncEntityBody{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "x", "status": "open"}}
	w := syncRequest(t, app, http.MethodPut, "/api/sync/entities/TKT-001", "", body)
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("blind push on existing: got %d, want 412 (%s)", w.Code, w.Body.String())
	}
}

// TestSync_Push422_UnknownType: invalid content (unknown entity type) is a hard
// validation error → 422, DISTINCT from the 412 conflict.
func TestSync_Push422_UnknownType(t *testing.T) {
	app := newHandlerTestApp(t)
	body := syncEntityBody{ID: "ZZ-1", Type: "no-such-type", Properties: map[string]any{"title": "x"}}
	w := syncRequest(t, app, http.MethodPut, "/api/sync/entities/ZZ-1", "", body)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid content: got %d, want 422 (%s)", w.Code, w.Body.String())
	}
}

// TestSync_PathTraversalRejected: an id with traversal/separator characters is
// rejected before reaching the store.
func TestSync_PathTraversalRejected(t *testing.T) {
	// The router would normalize some of these, so test the validator directly
	// for the cases that could slip through as a single segment.
	for _, bad := range []string{"..", "a..b", "a\\b"} {
		if validIDSegment(bad) {
			t.Errorf("validIDSegment(%q) = true, want false", bad)
		}
	}
	for _, good := range []string{"TKT-001", "REQ-abc", "a_b.c"} {
		if !validIDSegment(good) {
			t.Errorf("validIDSegment(%q) = false, want true", good)
		}
	}
}

// TestSync_DeleteEntity_HappyAnd412: delete with correct If-Match removes the
// record; delete with a stale If-Match conflicts.
func TestSync_DeleteEntity(t *testing.T) {
	app := newHandlerTestApp(t)

	e, err := app.store.GetEntity(t.Context(), "TKT-002")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	hash := canonical.HashEntity(*e)

	// Stale If-Match → 412.
	stale := syncRequest(t, app, http.MethodDelete, "/api/sync/entities/TKT-002", "deadbeef", nil)
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale delete: got %d, want 412 (%s)", stale.Code, stale.Body.String())
	}

	// Base-less delete (no If-Match) on an existing record → 412, NOT a blind
	// delete (symmetric with push; the record must still exist afterwards).
	blind := syncRequest(t, app, http.MethodDelete, "/api/sync/entities/TKT-002", "", nil)
	if blind.Code != http.StatusPreconditionFailed {
		t.Fatalf("base-less delete: got %d, want 412 (no blind delete)", blind.Code)
	}
	if _, err := app.store.GetEntity(t.Context(), "TKT-002"); err != nil {
		t.Fatal("base-less delete removed the record — must require If-Match")
	}

	// Correct If-Match → 200.
	w := syncRequest(t, app, http.MethodDelete, "/api/sync/entities/TKT-002", hash, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if _, err := app.store.GetEntity(t.Context(), "TKT-002"); err == nil {
		t.Fatal("entity still present after delete")
	}
}

// TestSync_ManifestUnsupportedOnNonPostgres: the memory test backend has no
// ManifestSince, so the manifest endpoint reports 501 (not a crash).
func TestSync_ManifestUnsupportedOnNonPostgres(t *testing.T) {
	app := newHandlerTestApp(t)
	if app.syncManifest() != nil {
		t.Skip("test backend unexpectedly supports the manifest")
	}
	w := syncRequest(t, app, http.MethodGet, "/api/sync/manifest", "", nil)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("manifest on non-pg backend: got %d, want 501 (%s)", w.Code, w.Body.String())
	}
}

// TestSync_AttributesToToolSync: a synced write is audited as Tool=sync (not
// data-entry), preserving the proxy-set User.
func TestSync_AttributesToToolSync(t *testing.T) {
	mem := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, mem)
	// Model the proxy-set principal: User=alice, Tool=data-entry (the resolver
	// always sets data-entry). The sync handler must re-stamp Tool=sync.
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})

	body := syncEntityBody{ID: "TKT-AUD", Type: "ticket", Properties: map[string]any{"title": "a", "status": "open"}}
	w := syncRequest(t, app, http.MethodPut, "/api/sync/entities/TKT-AUD", "", body)
	if w.Code != http.StatusOK {
		t.Fatalf("push: %d %s", w.Code, w.Body.String())
	}

	records := mem.Records()
	if len(records) == 0 {
		t.Fatal("no audit record for the sync write")
	}
	last := records[len(records)-1]
	if last.Principal.Tool != principal.ToolSync {
		t.Errorf("Tool = %q, want %q", last.Principal.Tool, principal.ToolSync)
	}
	if last.Principal.User != "alice" {
		t.Errorf("User = %q, want alice (proxy-set user must be preserved)", last.Principal.User)
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

	w := syncRequest(t, app, http.MethodGet, "/api/sync/manifest?cursor=4", "", nil)
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
		t.Errorf("relation id = %q, want slash triple form (matches the PUT path)", resp.Changes[2].ID)
	}

	// A cursor past everything returns no changes but keeps the cursor.
	w2 := syncRequest(t, app, http.MethodGet, "/api/sync/manifest?cursor=99", "", nil)
	var resp2 syncManifestResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if len(resp2.Changes) != 0 {
		t.Errorf("expected no changes past cursor, got %d", len(resp2.Changes))
	}
}

// syncGetAs drives handleSyncGet directly under the given principal's ACL
// read scope (mirroring getEntityAs — bypasses the router so the gate is
// attached explicitly via gateCtxFor).
func syncGetAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	kind, rest string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/sync/"+kind+"/"+rest, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleSyncGet(rec, req, kind, rest)
	return rec
}

// TestSync_GetEntity_ACLDenied pins the IB-review #1 fix: the sync entity GET
// applies the same read-ACL gate as every other read path. A principal without
// read on a type 404s (indistinguishable from not-found), and never receives
// the body or ETag of an entity it may not read.
func TestSync_GetEntity_ACLDenied(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := aliceCtx()

	// Allowed type → 200 with body + ETag.
	if rec := syncGetAs(ctx, t, app, d, "entities", "TKT-001"); rec.Code != http.StatusOK {
		t.Fatalf("GET allowed entity: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Denied type → 404, no ETag, no body leak.
	rec := syncGetAs(ctx, t, app, d, "entities", "FEAT-001")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET denied entity: got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if etag := rec.Header().Get("ETag"); etag != "" {
		t.Errorf("denied read leaked ETag %q", etag)
	}
	if body := rec.Body.String(); strings.Contains(body, "F1") {
		t.Errorf("denied read leaked entity body: %s", body)
	}
}

// TestSync_GetRelation_ACLDenied pins that a sync relation read follows the
// source entity's read visibility: a principal who cannot read the From entity's
// type cannot read the relation.
func TestSync_GetRelation_ACLDenied(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "CMP-1", Type: "component", Properties: map[string]any{"name": "Core"}})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "belongs_to", To: "CMP-1"})

	// alice may read components but NOT tickets — so the belongs_to relation
	// (sourced on a ticket) must be invisible to her.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"component"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := syncGetAs(aliceCtx(), t, app, d, "relations", "TKT-001/belongs_to/CMP-1")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET denied relation: got %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestSync_Manifest_ACLFiltered pins that the manifest drops rows the principal
// cannot read while still advancing the cursor past them (so a client never
// loops re-fetching hidden rows). Entity rows gate on their own type; the
// relation row gates on its source entity.
func TestSync_Manifest_ACLFiltered(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "F1"}})
	seedEntity(app, &entity.Entity{ID: "CMP-1", Type: "component", Properties: map[string]any{"name": "Core"}})

	app.store = manifestStore{Store: app.store, entries: []synctypes.ManifestEntry{
		{Kind: "e", IDA: "TKT-1", Typ: "ticket", Deleted: false, Seq: 5},
		{Kind: "e", IDA: "FEAT-1", Typ: "feature", Deleted: false, Seq: 6},  // hidden from alice
		{Kind: "e", IDA: "TKT-2", Typ: "ticket", Deleted: true, Seq: 7},     // ticket tombstone — visible
		{Kind: "r", IDA: "FEAT-1", IDB: "needs", IDC: "CMP-1", Seq: 8},      // sourced on feature — hidden
		{Kind: "r", IDA: "TKT-1", IDB: "belongs_to", IDC: "CMP-1", Seq: 9},  // sourced on ticket — visible
	}}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/sync/manifest?cursor=4", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleSyncManifest(rec, req)

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
	// Cursor still advances to the highest seq (9), INCLUDING the dropped
	// rows, so the client doesn't re-poll the hidden tail forever.
	if resp.Cursor != "9" {
		t.Errorf("cursor = %q, want 9 (highest seq over all rows, visible or not)", resp.Cursor)
	}
}

// TestSync_PushRelation round-trips a relation push.
func TestSync_PushRelation(t *testing.T) {
	app := newHandlerTestApp(t)
	// belongs_to: ticket -> component. Seed a component first.
	comp := &entity.Entity{ID: "CMP-1", Type: "component", Properties: map[string]any{"name": "Core"}}
	if _, err := app.syncApplierFor().ApplyEntity(t.Context(), comp); err != nil {
		t.Fatalf("seed component: %v", err)
	}
	body := syncRelationBody{From: "TKT-001", Type: "belongs_to", To: "CMP-1"}
	w := syncRequest(t, app, http.MethodPut, "/api/sync/relations/TKT-001/belongs_to/CMP-1", "", body)
	if w.Code != http.StatusOK {
		t.Fatalf("push relation: %d %s", w.Code, w.Body.String())
	}
	if _, err := app.store.GetRelation(t.Context(), "TKT-001", "belongs_to", "CMP-1"); err != nil {
		t.Fatalf("relation not persisted: %v", err)
	}
}
