package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestV1Schema_IntegerRange pins that an integer's declared min and max
// reach `/_schema`, where the SPA puts them on the number input
// (TKT-BRAM1A).
func TestV1Schema_IntegerRange(t *testing.T) {
	app := newTestAppV1(t)
	rec := httptest.NewRecorder()
	app.handleV1Schema(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var schema v1.Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatal(err)
	}
	points := schema.Entities["ticket"].Properties["points"]
	if points.Min == nil || *points.Min != 1 || points.Max == nil || *points.Max != 5 {
		t.Errorf("points range = %v..%v, want 1..5", points.Min, points.Max)
	}
	if title := schema.Entities["ticket"].Properties["title"]; title.Min != nil || title.Max != nil {
		t.Errorf("title carries a range: %v..%v", title.Min, title.Max)
	}
}

// TestV1UpdateEntity_OutOfRangeWarns pins the write policy for a value
// outside the declared range: a hand-edit can produce it, so the write
// succeeds with a property_value_invalid warning (DEC-HWZHA), not a 422.
func TestV1UpdateEntity_OutOfRangeWarns(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Original"},
	})

	code, resp := patchTicketJSON(t, app, `{"properties":{"points":7}}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH returned %d, want 200", code)
	}
	found := false
	for _, w := range resp.Warnings {
		if w.Code == "property_value_invalid" && w.Path == "/properties/points" {
			found = true
		}
	}
	if !found {
		t.Errorf("want property_value_invalid at /properties/points; warnings=%+v", resp.Warnings)
	}

	code, body := createTicketRaw(t, app, `{"properties":{"title":"New","points":0}}`)
	if code != http.StatusCreated {
		t.Fatalf("POST returned %d, want 201; body=%s", code, body)
	}
	if !strings.Contains(body, `"code":"property_value_invalid"`) || !strings.Contains(body, `"/properties/points"`) {
		t.Errorf("create: want property_value_invalid at /properties/points; body=%s", body)
	}

	code, resp = patchTicketJSON(t, app, `{"properties":{"points":3}}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH returned %d, want 200", code)
	}
	for _, w := range resp.Warnings {
		if w.Path == "/properties/points" {
			t.Errorf("in-range value warned: %+v", w)
		}
	}
}
