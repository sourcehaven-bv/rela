package dataentry

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestACLWrite_PatchOnHiddenIs404 pins AC3: PATCH on a hidden entity
// returns 404 with same body shape as GET-on-nonexistent. Crucially
// the gate runs BEFORE body parse, so even a malformed body must 404
// (not 400) — otherwise the 400 confirms the URL exists.
func TestACLWrite_PatchOnHiddenIs404(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"none": {}},
		Assignments: map[string]string{"bob": "none"},
	}, app)
	app.acl = d
	bobCtx := principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolDataEntry})

	// Valid body, hidden target — must 404.
	rec := patchEntityAs(bobCtx, t, app, d, "ticket", "tickets", "TKT-001",
		`{"properties":{"title":"hack"}}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH hidden, valid body: got %d, want 404 (saw %s)", rec.Code, rec.Body)
	}

	// Malformed JSON body, hidden target — must STILL 404 (not 400).
	// Otherwise the 400 is an existence oracle: "URL is valid but body
	// is wrong" tells the attacker the entity exists. RR-FGUZ.
	rec = patchEntityAs(bobCtx, t, app, d, "ticket", "tickets", "TKT-001",
		`{`, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH hidden, malformed body: got %d, want 404 (RR-FGUZ); body=%s", rec.Code, rec.Body)
	}

	// Stale If-Match on hidden target — must STILL 404 (not 412).
	rec = patchEntityAs(bobCtx, t, app, d, "ticket", "tickets", "TKT-001",
		`{"properties":{"title":"hack"}}`,
		http.Header{"If-Match": []string{`"stale-etag"`}})
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH hidden, stale If-Match: got %d, want 404 (RR-FGUZ); body=%s", rec.Code, rec.Body)
	}
}

// TestACLWrite_DeleteOnHiddenIs404 pins AC3 for DELETE.
func TestACLWrite_DeleteOnHiddenIs404(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"none": {}},
		Assignments: map[string]string{"bob": "none"},
	}, app)
	app.acl = d
	bobCtx := principal.With(context.Background(), principal.Principal{User: "bob", Tool: principal.ToolDataEntry})

	rec := deleteEntityAs(bobCtx, t, app, d, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusNotFound {
		t.Errorf("DELETE hidden: got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "/errors/not_found") {
		t.Errorf("DELETE deny body missing not_found code: %s", rec.Body)
	}
}

// (Visible-but-write-denied → 403 is preserved by AuthorizeWrite in
// entitymanager; covered by `internal/entitymanager/acl_test.go`.
// The dataentry-level test here would require wiring the test
// entitymanager with the same ACL, which `newTestAppV1` does not do.
// What this PR pins is that the per-entity Visible gate runs BEFORE
// AuthorizeWrite — the cases that matter for that ordering are the
// two hidden-target tests above.)

// ---- helpers ----

func patchEntityAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID, body string, hdr http.Header,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/"+plural+"/"+entityID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, typeName, plural, entityID)
	return rec
}

func deleteEntityAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/"+plural+"/"+entityID, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1DeleteEntity(rec, req, typeName, plural, entityID)
	return rec
}
