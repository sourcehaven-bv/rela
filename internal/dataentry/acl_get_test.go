package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestACLGet_TypeLevelReadGrant pins AC1: a role with `read: [ticket]`
// lets the principal GET tickets but not features. Both 200 and 404
// emit the same wire shape — the 404 body for "denied" is identical
// to "nonexistent" (same `not_found` code).
func TestACLGet_TypeLevelReadGrant(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app)
	app.acl = d
	ctx := aliceCtx()

	// Visible.
	rec := getEntityAs(ctx, t, app, d, "ticket", "tickets", "TKT-001", "")
	if rec.Code != http.StatusOK {
		t.Errorf("GET ticket: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Denied.
	rec = getEntityAs(ctx, t, app, d, "feature", "features", "FEAT-001", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET feature (denied): got %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/errors/not_found") {
		t.Errorf("deny body missing not_found error code: %s", rec.Body)
	}

	// Parity: GET on a nonexistent id of an allowed type → byte-equal
	// body modulo the URL `instance` field, which legitimately differs.
	recNX := getEntityAs(ctx, t, app, d, "ticket", "tickets", "TKT-NONEXISTENT", "")
	if recNX.Code != http.StatusNotFound {
		t.Fatalf("GET nonexistent: got %d, want 404", recNX.Code)
	}
	if !strings.Contains(recNX.Body.String(), "/errors/not_found") {
		t.Errorf("nonexistent body missing not_found error code: %s", recNX.Body)
	}
	deniedShape := stripInstance(rec.Body.String())
	nonexistentShape := stripInstance(recNX.Body.String())
	if deniedShape != nonexistentShape {
		t.Errorf("denied vs nonexistent body shape differs:\n  denied:      %s\n  nonexistent: %s",
			deniedShape, nonexistentShape)
	}
}

// stripInstance removes the "instance" JSON field (which legitimately
// differs between a denied id and a nonexistent id since both reach
// different URLs) so the rest of the body can be compared for parity.
func stripInstance(s string) string {
	i := strings.Index(s, `,"instance":`)
	if i < 0 {
		return s
	}
	j := strings.IndexByte(s[i+1:], '}')
	if j < 0 {
		return s
	}
	return s[:i] + s[i+1+j:]
}

// TestACLGet_ETagSuppressedOnDeny pins AC5: a denied GET MUST NOT
// emit an ETag header and MUST NOT honor `If-None-Match` (always
// 404, never 304). Otherwise Alice's ETag replayed by Bob would
// surface 304 — confirming existence.
func TestACLGet_ETagSuppressedOnDeny(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer-tickets": {Read: []string{"ticket"}},
			"viewer-empty":   {},
		},
		Assignments: map[string]string{"alice": "viewer-tickets", "bob": "viewer-empty"},
	}, app)
	app.acl = d

	// Alice gets a valid response with an ETag.
	aliceRec := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "")
	if aliceRec.Code != http.StatusOK {
		t.Fatalf("alice GET: %d %s", aliceRec.Code, aliceRec.Body)
	}
	etag := aliceRec.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("alice GET missing ETag")
	}

	// Bob is denied — no ETag header, no 304 even with alice's ETag.
	bobCtx := principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
	bobRec := getEntityAsWithHeaders(bobCtx, t, app, d, "ticket", "tickets", "TKT-001", "",
		http.Header{"If-None-Match": []string{etag}})
	if bobRec.Code != http.StatusNotFound {
		t.Errorf("bob GET with alice's ETag: got %d, want 404 (not 304)", bobRec.Code)
	}
	if bobRec.Header().Get("ETag") != "" {
		t.Errorf("bob deny response emitted ETag %q; want absent", bobRec.Header().Get("ETag"))
	}
}

// TestACLGet_IncludeFilter pins AC4: `?include=*` MUST omit hidden
// neighbors. Without the include-gate, an attacker enumerates the
// graph via the include channel even when GET on the neighbor's
// type would 404.
func TestACLGet_IncludeFilter(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app)
	app.acl = d

	rec := getEntityAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "include=*")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET with include=*: %d %s", rec.Code, rec.Body)
	}
	// The relations map naming the edge target is metadata, not the
	// hidden entity itself (gating-by-id is the SPA's responsibility
	// per dataentry/CLAUDE.md). What MUST NOT leak is the serialized
	// neighbor entity in the `included` map.
	body := rec.Body.String()
	if strings.Contains(body, `"included":`) && strings.Contains(body, `"FEAT-001":`) {
		t.Errorf("hidden FEAT-001 leaked via included map; body=%s", body)
	}
}

// ---- helpers ----

func aliceCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
}

// mustNewACL constructs a *acl.Declarative for the given policy using
// the app's store as both Graph and GraphQueryer. Mirrors production
// wiring (appbuild) so tests exercise the same paths.
func mustNewACL(t *testing.T, p *acl.Policy, app *App) *acl.Declarative {
	t.Helper()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(app.store), app.store)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	return d
}

// gateCtxFor attaches the readGate + acl.Request to ctx, mirroring
// what the production middleware does. Test handlers bypass the
// middleware so they need this explicit setup.
func gateCtxFor(ctx context.Context, t *testing.T, d *acl.Declarative) context.Context {
	t.Helper()
	req, err := d.ForPrincipal(principal.From(ctx))
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	ctx = acl.WithRequest(ctx, req)
	return withReadGate(ctx, aclReadGate{req: req})
}

func getEntityAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID, rawQuery string,
) *httptest.ResponseRecorder {
	t.Helper()
	return getEntityAsWithHeaders(ctx, t, app, d, typeName, plural, entityID, rawQuery, nil)
}

func getEntityAsWithHeaders(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID, rawQuery string, hdr http.Header,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/" + plural + "/" + entityID
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, typeName, plural, entityID)
	return rec
}
