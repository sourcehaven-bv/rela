package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// TestV1CreateEntity_NoInventedStatus pins that a create through the HTTP API
// (the web form path) does not invent a status the schema does not declare
// (BUG-ZD4PIN). "feature" has no status property; "ticket" has one with no
// default.
func TestV1CreateEntity_NoInventedStatus(t *testing.T) {
	app := newTestAppV1(t)
	for _, tc := range []struct{ typ, plural string }{
		{"feature", "features"},
		{"ticket", "tickets"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/"+tc.plural,
			strings.NewReader(`{"properties":{"title":"New"}}`))
		rec := httptest.NewRecorder()
		app.write.handleV1CreateEntity(rec, req, tc.typ, tc.plural)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: POST returned %d: %s", tc.typ, rec.Code, rec.Body.String())
		}
		var created v1.Entity
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("%s: decode: %v", tc.typ, err)
		}
		if got, ok := created.Properties["status"]; ok {
			t.Errorf("%s: status = %v, want absent", tc.typ, got)
		}
	}
}
