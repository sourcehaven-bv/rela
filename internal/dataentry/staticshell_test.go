package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStaticShell_ServesOnlyEmbeddedBytes pins that the tenant-free shell
// answers every path from the binary: the shell for a client route and for an
// API path alike, never a handler that could read a store.
func TestStaticShell_ServesOnlyEmbeddedBytes(t *testing.T) {
	if err := CheckEmbeddedSPA(); err != nil {
		t.Skipf("embedded SPA not built: %v", err)
	}
	h := StaticShell()
	shell := httptest.NewRecorder()
	h.ServeHTTP(shell, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if shell.Code != http.StatusOK || shell.Body.Len() == 0 {
		t.Fatalf("GET / = %d with %d bytes, want the shell", shell.Code, shell.Body.Len())
	}
	for _, p := range []string{"/tickets/TKT-1", "/api/v1/tickets"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, http.NoBody))
		if w.Code != http.StatusOK || w.Body.String() != shell.Body.String() {
			t.Errorf("GET %s = %d, want the shell bytes", p, w.Code)
		}
	}
}
