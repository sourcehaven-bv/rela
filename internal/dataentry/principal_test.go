package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

// TestStampAuditPrincipal_DefaultResolver verifies that every request
// flowing through the middleware with the default resolver carries
// Principal{User:"unknown", Tool:"data-entry"} (per design-review:
// the server's $USER would be misleading for human edits;
// per-request override lands in a follow-up).
//
// Satisfies AC4 for the data-entry entry point.
func TestStampAuditPrincipal_DefaultResolver(t *testing.T) {
	var captured audit.Principal
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = audit.PrincipalFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := stampAuditPrincipal(captureHandler, defaultPrincipalResolver)

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

// TestStampAuditPrincipal_CustomResolver verifies the seam works for
// the follow-up PR: a header-aware resolver returns a per-request
// Principal that the middleware stamps on the ctx.
func TestStampAuditPrincipal_CustomResolver(t *testing.T) {
	resolver := func(r *http.Request) audit.Principal {
		return audit.Principal{
			User: r.Header.Get("X-Test-User"),
			Tool: audit.ToolDataEntry,
		}
	}

	var captured audit.Principal
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = audit.PrincipalFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := stampAuditPrincipal(captureHandler, resolver)

	req := httptest.NewRequest(http.MethodGet, "/anything", http.NoBody)
	req.Header.Set("X-Test-User", "alice")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured.User != "alice" {
		t.Errorf("User = %q, want 'alice' (resolver should read header)", captured.User)
	}
	if captured.Tool != audit.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", captured.Tool, audit.ToolDataEntry)
	}
}
