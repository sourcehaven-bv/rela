// Tests for TKT-E6094: PATCH /api/v1/{plural}/{id} accepts
// `properties_unset: []string` to delete declared properties.
// Mirrors the user intent "I cleared this field" distinct from
// "I didn't touch this field" — autosave needs the difference.

package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// patchAndDecode is a tiny helper that PATCHes the ticket and returns
// the parsed response body so individual tests can assert on it. The
// existing TestV1UpdateEntity_* fixtures don't expose this pattern.
type updateResponse struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Warnings   []Warning              `json:"warnings,omitempty"`
}

func patchTicketJSON(t *testing.T, app *App, body string) (int, updateResponse) {
	t.Helper()
	const entityID = "TKT-001"
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/"+entityID, strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", entityID)
	var resp updateResponse
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	}
	return rec.Code, resp
}

// patchTicketRaw returns the raw response body for assertions that
// need to inspect non-V1Entity-shaped responses (e.g. 403 affordance
// denials).
func patchTicketRaw(t *testing.T, app *App, body string) (code int, respBody string) {
	t.Helper()
	const entityID = "TKT-001"
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/"+entityID, strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", entityID)
	return rec.Code, rec.Body.String()
}

// AC15: PATCH with `properties_unset` removes the named keys.
func TestV1UpdateEntity_PropertiesUnset_RemovesKeys(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	// Use `status` (declared) so the test exercises the happy path
	// without triggering the unknown-key warning.
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Original",
			"status": "open",
		},
	})

	code, resp := patchTicketJSON(t, app,
		`{"properties_unset":["status"]}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH returned %d", code)
	}
	if _, present := resp.Properties["status"]; present {
		t.Errorf("expected status to be removed, got %v", resp.Properties["status"])
	}
	// Untouched properties survive.
	if resp.Properties["title"] != "Original" {
		t.Errorf("title was clobbered: got %v", resp.Properties["title"])
	}
	for _, w := range resp.Warnings {
		if w.Code == "unknown_property_unset_key" {
			t.Errorf("declared key should not warn, got %+v", w)
		}
	}
}

// AC16: PATCH with `properties_unset` of an undeclared key produces a
// `unknown_property_unset_key` warning (200 + warning, DEC-HWZHA).
// TKT-G7N5 F8: unknown property unset keys are now rejected with 403
// (rule_id=field-affordance:hidden) — the same shape as a true
// hidden-field rejection. This closes the side channel where unknown
// vs hidden produced distinguishable responses. The previous
// behavior was a 200 with an `unknown_property_unset_key` warning;
// that behavior is incompatible with the affordance-parity invariant
// and was removed.
func TestV1UpdateEntity_PropertiesUnset_UnknownKey_Forbidden(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})

	code, body := patchTicketRaw(t, app,
		`{"properties_unset":["nonexistent_property"]}`)
	if code != http.StatusForbidden {
		t.Fatalf("PATCH returned %d, want 403; body=%s", code, body)
	}
	if !strings.Contains(body, `"rule_kind":"affordance"`) {
		t.Errorf("body should name rule_kind=affordance; got: %s", body)
	}
	if !strings.Contains(body, `"rule_id":"field-affordance:hidden:nonexistent_property"`) {
		t.Errorf("body should contain hidden-shaped rule_id; got: %s", body)
	}
}

// AC17: PATCH with BOTH `properties` AND `properties_unset` applies in
// order — upsert first, then delete. The order matters when the same
// body sets X and unsets Y in one round-trip.
func TestV1UpdateEntity_PropertiesAndUnset_Together(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Original",
			"status": "open",
		},
	})

	code, resp := patchTicketJSON(t, app,
		`{"properties":{"title":"Updated"},"properties_unset":["status"]}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH returned %d", code)
	}
	if resp.Properties["title"] != "Updated" {
		t.Errorf("expected title=Updated, got %v", resp.Properties["title"])
	}
	if _, present := resp.Properties["status"]; present {
		t.Error("expected status removed")
	}
}

// AC18: PATCH with properties + properties_unset + relations all in one
// body applies in order (upsert → unset → entity write → relations).
func TestV1UpdateEntity_PropertiesUnsetAndRelations_Together(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Original",
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{
		ID:         "FEAT-001",
		Type:       "feature",
		Properties: map[string]interface{}{"title": "Feature"},
	})

	body := `{
		"properties":{"title":"Updated"},
		"properties_unset":["status"],
		"relations":{"blocks":{"data":[{"type":"feature","id":"FEAT-001"}]}}
	}`
	code, resp := patchTicketJSON(t, app, body)
	if code != http.StatusOK {
		t.Fatalf("PATCH returned %d: response %+v", code, resp)
	}
	if resp.Properties["title"] != "Updated" {
		t.Errorf("title not updated: %v", resp.Properties["title"])
	}
	if _, present := resp.Properties["status"]; present {
		t.Error("status not removed")
	}
	// Relation written
	edges := app.reader.outgoingRelations(context.Background(), "TKT-001")
	if len(edges) != 1 || edges[0].Type != "blocks" || edges[0].To != "FEAT-001" {
		t.Errorf("expected one blocks→FEAT-001 edge, got %+v", edges)
	}
}

// Defensive: PATCH with properties_unset for a property that is
// declared in the metamodel but absent from the entity (e.g., never
// set) is a silent no-op — no warning, no error.
func TestV1UpdateEntity_PropertiesUnset_AbsentDeclaredKey_Silent(t *testing.T) {
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())

	// Seed with title only; `status` is declared on `ticket` but absent.
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x"},
	})

	code, resp := patchTicketJSON(t, app,
		`{"properties_unset":["status"]}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH returned %d", code)
	}
	for _, w := range resp.Warnings {
		if w.Code == "unknown_property_unset_key" {
			t.Errorf("did not expect unknown_property_unset_key for declared key, got %+v", w)
		}
	}
}
