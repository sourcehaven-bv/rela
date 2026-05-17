package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

// TestStampAuditPrincipal verifies that every request flowing through
// the data-entry router carries a Principal stamped with Tool=data-entry
// and User=unknown (per design-review: the server's $USER would be
// misleading for human edits; per-request override lands in a follow-up).
//
// Satisfies AC4 for the data-entry entry point.
func TestStampAuditPrincipal(t *testing.T) {
	var captured audit.Principal
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = audit.PrincipalFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := stampAuditPrincipal(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/anything", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured.Tool != audit.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", captured.Tool, audit.ToolDataEntry)
	}
	if captured.User != "unknown" {
		t.Errorf("User = %q, want 'unknown' (per design-review: per-request override is a follow-up)",
			captured.User)
	}
}
