package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
	}, app.store)
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
	deniedShape := stripInstance(t, rec.Body.String())
	nonexistentShape := stripInstance(t, recNX.Body.String())
	if deniedShape != nonexistentShape {
		t.Errorf("denied vs nonexistent body shape differs:\n  denied:      %s\n  nonexistent: %s",
			deniedShape, nonexistentShape)
	}
}

// stripInstance returns a canonical-JSON form of an error response
// with the "instance" field cleared. The URL legitimately differs
// between "denied" and "nonexistent" parity cases (each reaches its
// own URL); other fields must match.
//
// Parsing + re-encoding (instead of string slicing — RR-QLQW) makes
// the comparator robust against future v1.Error field additions and
// against JSON-encoder reordering quirks.
func stripInstance(t *testing.T, s string) string {
	t.Helper()
	var v v1.Error
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("stripInstance: invalid JSON %q: %v", s, err)
	}
	v.Instance = ""
	out, err := json.Marshal(&v)
	if err != nil {
		t.Fatalf("stripInstance: re-encode: %v", err)
	}
	return string(out)
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
	}, app.store)
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
	}, app.store)
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

// TestACLGet_WriteGateErrorMapping pins the error mapping in
// writeGateError (RR-J25J): context.Canceled emits nothing (client
// has disconnected); context.DeadlineExceeded surfaces 504;
// everything else 500 with the acl_query_failed code. Drives each
// branch via a fakeGate whose PermitsRead returns a configured error.
//
// The raw backend error string MUST NOT appear in any response body
// (information exposure — a pg error can name tables/columns). The
// detail is the constant "check server logs"; the real error goes to
// slog only. Same RR-372L pattern as attachACLRequest; required by
// the IB review on PR 939 (POLICY-015 §3).
func TestACLGet_WriteGateErrorMapping(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	// Deliberately shaped like a pg error: leaking it would put the
	// fake table name on the wire, which the assertions below catch.
	syntheticErr := errors.New(`pq: relation "secret_internal_table" does not exist`)

	cases := []struct {
		name      string
		err       error
		wantCode  int
		wantBody  string // substring; empty means no body
		emptyResp bool   // true: handler MUST NOT write a response
	}{
		{
			name:      "canceled emits nothing",
			err:       context.Canceled,
			wantCode:  http.StatusOK, // ResponseRecorder default when WriteHeader is never called
			emptyResp: true,
		},
		{
			name:     "deadline exceeded surfaces 504",
			err:      context.DeadlineExceeded,
			wantCode: http.StatusGatewayTimeout,
			wantBody: "acl_query_timeout",
		},
		{
			name:     "generic err surfaces 500",
			err:      syntheticErr,
			wantCode: http.StatusInternalServerError,
			wantBody: "acl_query_failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withReadGate(context.Background(), fakeGate{permitsErr: tc.err})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")

			if rec.Code != tc.wantCode {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.emptyResp {
				if rec.Body.Len() != 0 {
					t.Errorf("client-canceled path wrote body %q; want empty", rec.Body.String())
				}
				return
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantBody) {
				t.Errorf("body %q missing substring %q", body, tc.wantBody)
			}
			if strings.Contains(body, "secret_internal_table") {
				t.Errorf("raw backend error leaked into response body: %s", body)
			}
			if !strings.Contains(body, "check server logs") {
				t.Errorf("body %q missing constant detail %q", body, "check server logs")
			}
		})
	}
}

// ---- helpers ----

// fakeGate is a readGate impl that returns configured errors from
// PermitsRead / PermitsReadMany. Used by error-mapping tests;
// production sites use aclReadGate.
type fakeGate struct {
	permitsErr error
}

func (g fakeGate) PermitsRead(context.Context, string, string) (bool, error) {
	return false, g.permitsErr
}

func (g fakeGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	if g.permitsErr != nil {
		return nil, g.permitsErr
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = false
	}
	return m, nil
}

func (g fakeGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{DenyAll: true}
}

func (g fakeGate) SearchScope(context.Context, []string) map[string]search.TypeScope {
	return nil // all-deny, matching ReadQuery above
}

// principalCtx returns a context carrying a stamped data-entry
// principal for `user`. RR-MILH: replaces the previous parameterless
// `aliceCtx()` helper so tests that want a different principal
// (bob, charlie, …) don't fork between "use the helper" and "build
// inline" styles.
func principalCtx(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
}

// aliceCtx is a back-compat alias for principalCtx("alice"). Callers
// that don't care about the user can keep using it; new tests should
// prefer principalCtx for clarity.
func aliceCtx() context.Context { return principalCtx("alice") }

// mustNewACL constructs a *acl.Declarative for the given policy using
// the supplied store as both Graph and GraphQueryer. Mirrors production
// wiring (appbuild) so tests exercise the same paths. RR-AGSR: takes
// `store.Store` directly instead of `*App` so the dependency surface
// is explicit.
func mustNewACL(t *testing.T, p *acl.Policy, st store.Store) *acl.Declarative {
	t.Helper()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
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
