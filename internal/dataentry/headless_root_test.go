package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHeadlessRoot pins the root handler of a headless build: `/` points to
// the API, every other path is 404. It runs in every build; the routing that
// selects it is covered by TestHeadless_Router under the headless tag.
func TestHeadlessRoot(t *testing.T) {
	h := withSpecLink(headlessRoot())
	const link = `</api/v1/_openapi.json>; rel="service-desc"`
	for _, tc := range []struct {
		name, method, path string
		code               int
		body, link         string
	}{
		{"get root", http.MethodGet, "/", http.StatusOK, headlessRootBody, link},
		{"head root", http.MethodHead, "/", http.StatusOK, "", link},
		{"post root", http.MethodPost, "/", http.StatusNotFound, "", ""},
		{"index.html", http.MethodGet, "/index.html", http.StatusNotFound, "", ""},
		{"spa route", http.MethodGet, "/tickets/TKT-1", http.StatusNotFound, "", ""},
		{"asset", http.MethodGet, "/assets/index-abc.js", http.StatusNotFound, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, http.NoBody))
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d", rec.Code, tc.code)
			}
			if got := rec.Header().Get("Link"); got != tc.link {
				t.Errorf("Link = %q, want %q", got, tc.link)
			}
			if tc.code == http.StatusOK && rec.Body.String() != tc.body {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.body)
			}
		})
	}
}
