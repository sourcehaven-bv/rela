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
	}, app.store)
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

	// Valid If-Match (current ETag) on hidden target — must STILL 404
	// (RR-H9QB). An attacker who somehow obtained the current ETag
	// (cached from a prior probe, replayed from a log, leaked via a
	// side channel) MUST NOT get a 412 confirming the entity exists
	// nor a 200 letting them write through. The gate runs BEFORE
	// If-Match.
	currentETag := computeETagForTest(t, app, "TKT-001")
	rec = patchEntityAs(bobCtx, t, app, d, "ticket", "tickets", "TKT-001",
		`{"properties":{"title":"hack"}}`,
		http.Header{"If-Match": []string{currentETag}})
	if rec.Code != http.StatusNotFound {
		t.Errorf("PATCH hidden, current If-Match: got %d, want 404 (RR-H9QB); body=%s", rec.Code, rec.Body)
	}
}

// computeETagForTest re-derives the ETag the handler would emit for
// the given entity. Used by RR-H9QB to construct a "valid" If-Match
// without first having to GET the entity (the attacker scenario the
// test models would obtain the ETag through a side channel).
func computeETagForTest(t *testing.T, app *App, entityID string) string {
	t.Helper()
	e, found := app.reader.getEntity(t.Context(), entityID)
	if !found {
		t.Fatalf("computeETagForTest: %s not in store", entityID)
	}
	return app.computeEntityETag(t.Context(), e)
}

// TestACLWrite_DeleteOnHiddenIs404 pins AC3 for DELETE.
func TestACLWrite_DeleteOnHiddenIs404(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"none": {}},
		Assignments: map[string]string{"bob": "none"},
	}, app.store)
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

// patchEntityAs runs handleV1UpdateEntity with the gate ctx attached.
// typeName/plural are accepted for symmetry with deleteEntityAs and
// getEntityAs; today's tests only exercise tickets, but the helper
// surface keeps the door open to fixtures with other types.
//
//nolint:unparam // typeName always "ticket" today — see godoc.
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
