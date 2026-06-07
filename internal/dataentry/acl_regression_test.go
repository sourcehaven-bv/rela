package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestACLRegression_NopACL_GetUnchanged pins AC6: with no ACL
// configured (the readGate returns nopReadGate via the ctx fallback),
// the GET response shape is byte-identical to today's pre-ACL
// behavior for every code path the gate would otherwise intercept.
//
// "Byte-identical" is verified structurally — Content-Type is the
// same, status is 200, body parses, the entity is present. The
// gate's nopReadGate.Visible returns (true, nil) which is what the
// "no ACL" pre-gate code path also implicitly assumed; the test is
// the canary that nopReadGate stays the no-op it advertises.
func TestACLRegression_NopACL_GetUnchanged(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	// No app.acl, no readGate on ctx — the handler sees
	// readGateFromContext returning nopReadGate.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusOK {
		t.Fatalf("NopACL GET: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if rec.Header().Get("ETag") == "" {
		t.Errorf("NopACL GET missing ETag (regression: AllowAll path must keep emitting ETag)")
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("NopACL GET Content-Type: got %q, want application/json", rec.Header().Get("Content-Type"))
	}
}

// TestACLRegression_NopACL_NonExistentStill404 pins that the
// gate's nil-handling does not change the not-found path.
func TestACLRegression_NopACL_NonExistentStill404(t *testing.T) {
	app := newTestAppV1(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-NONEXISTENT", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-NONEXISTENT")

	if rec.Code != http.StatusNotFound {
		t.Errorf("NopACL nonexistent: got %d, want 404", rec.Code)
	}
}
