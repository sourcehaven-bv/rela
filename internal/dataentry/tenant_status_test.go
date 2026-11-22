package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestTenantStatus_SingleTenant pins the single-tenant answer of
// GET /api/v1/_tenant: active and writable, so the SPA's start-up read gets
// a 200 instead of a 404 the browser logs as an error (BUG-ZL4K7QW).
func TestTenantStatus_SingleTenant(t *testing.T) {
	router := newHandlerTestApp(t).NewRouter()

	tests := []struct {
		name   string
		method string
		want   int
	}{
		{"get answers active", http.MethodGet, http.StatusOK},
		{"other methods are refused", http.MethodPost, http.StatusMethodNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, TenantStatusPath, http.NoBody))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if tc.want != http.StatusOK {
				if got := rec.Header().Get("Allow"); got != http.MethodGet {
					t.Errorf("Allow = %q, want GET", got)
				}
				return
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["status"] != "active" || len(body) != 1 {
				t.Errorf("body = %v, want only status active", body)
			}
		})
	}
}
