package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAsOf_RefusesWhatItCannotServe pins the refusals of attachAsOf. The
// test app runs on memstore, which keeps no history, so a well-formed read
// reaches the backend check and gets 501.
func TestAsOf_RefusesWhatItCannotServe(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), newFixture())

	cases := []struct {
		name   string
		method string
		target string
		status int
		code   string
	}{
		{"not a timestamp", http.MethodGet, "/api/v1/tickets?as_of=yesterday", http.StatusBadRequest, "as_of_invalid"},
		{"no zone", http.MethodGet, "/api/v1/tickets?as_of=2025-12-31T23:59:59", http.StatusBadRequest, "as_of_invalid"},
		{"twice", http.MethodGet, "/api/v1/tickets?as_of=2025-12-31T23:59:59Z&as_of=2025-12-30T00:00:00Z",
			http.StatusBadRequest, "as_of_duplicated"},
		{"write", http.MethodPost, "/api/v1/tickets?as_of=2025-12-31T23:59:59Z", http.StatusUnprocessableEntity,
			"as_of_read_only"},
		{"endpoint without history", http.MethodGet, "/api/v1/_analyze?as_of=2025-12-31T23:59:59Z",
			http.StatusUnprocessableEntity, "as_of_unsupported"},
		{"backend without history", http.MethodGet, "/api/v1/tickets?as_of=2025-12-31T23:59:59Z",
			http.StatusNotImplemented, "as_of_backend_unsupported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, httptest.NewRequest(tc.method, tc.target, http.NoBody))
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), tc.code)
		})
	}
}

func TestAsOfCapablePath(t *testing.T) {
	cases := map[string]bool{
		"/api/v1/tickets":                    true,
		"/api/v1/tickets/TKT-1":              true,
		"/api/v1/_views/ticket/TKT-1":        true,
		"/api/v1/_dashboard/card":            true,
		"/api/v1/_search":                    true,
		"/api/v1/_analyze":                   false,
		"/api/v1/tickets/_export":            false,
		"/api/v1/tickets/TKT-1/x":            false,
		"/api/v1/tickets/TKT-1/relations":    true,
		"/api/v1/tickets/TKT-1/mentioned-by": false,
		"/api/v1/":                           false,
		"/other":                             false,
	}
	for path, want := range cases {
		require.Equal(t, want, asOfCapablePath(path), path)
	}
}
