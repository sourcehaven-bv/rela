//go:build headless

package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHeadless_Router pins what a headless build serves: the API as usual,
// a text pointer at `/`, and 404 for every UI path.
func TestHeadless_Router(t *testing.T) {
	if !Headless {
		t.Fatal("Headless = false under the headless build tag")
	}
	router := newTestAppV1(t).NewRouter()
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{http.MethodGet, "/api/v1/_schema", http.StatusOK},
		{http.MethodGet, "/api/v1/_openapi.json", http.StatusOK},
		{http.MethodGet, "/", http.StatusOK},
		{http.MethodHead, "/", http.StatusOK},
		{http.MethodGet, "/index.html", http.StatusNotFound},
		{http.MethodGet, "/assets/index-abc.js", http.StatusNotFound},
		{http.MethodGet, "/static/favicon.svg", http.StatusNotFound},
		{http.MethodGet, "/tickets/TKT-1", http.StatusNotFound},
		{http.MethodGet, "/_custom/custom.css", http.StatusNotFound},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, http.NoBody))
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d; body %q", rec.Code, tc.code, rec.Body.String())
			}
		})
	}
}

// TestHeadless_NoEmbeddedUI pins that a headless build compiles in no UI
// bytes, and that the startup check and the tenant-free shell agree.
func TestHeadless_NoEmbeddedUI(t *testing.T) {
	if err := CheckEmbeddedSPA(); err == nil {
		t.Error("CheckEmbeddedSPA() = nil, want an error: a headless build embeds no SPA")
	}
	if id := EmbeddedBuildID(); id != "" {
		t.Errorf("EmbeddedBuildID() = %q, want empty", id)
	}
	if b := appEditorSource(); b != nil {
		t.Errorf("editor bundle embedded (%d bytes), want none", len(b))
	}
	for path, want := range map[string]int{
		"/":                    http.StatusOK,
		"/static/favicon.svg":  http.StatusNotFound,
		"/assets/index-abc.js": http.StatusNotFound,
	} {
		rec := httptest.NewRecorder()
		StaticShell().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		if rec.Code != want {
			t.Errorf("StaticShell %s = %d, want %d", path, rec.Code, want)
		}
	}
}
