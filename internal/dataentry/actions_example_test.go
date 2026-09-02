package dataentry

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestIcingaAlertExample runs the SHIPPED examples/icinga-alert.lua end to end
// through the real action handler.
//
// The example is documentation people copy, so an example that does not run is
// worse than no example: it teaches an API that does not exist. Running it here
// also means a rename in the Lua bindings breaks a test rather than silently
// rotting the file.
func TestIcingaAlertExample(t *testing.T) {
	script := readExampleAction(t, "icinga-alert.lua")

	app := newActionTestApp(t, map[string]string{"icinga-alert.lua": script})
	withIncidentType(app)
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"icinga-alert": {
			Script: "icinga-alert.lua",
			Request: &dataentryconfig.ActionRequest{
				Body:    true,
				Headers: []string{"X-Icinga-Event"},
			},
		},
	}

	alert := `{"host":"web1","service":"http","state":"CRITICAL","output":"connection refused"}`

	// First delivery creates the incident.
	rec := postRequestAction(t, app, "icinga-alert", alert, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first delivery: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	first := decodeIncidentResult(t, rec.Body.String())
	if first["status"] != "created" {
		t.Fatalf("first delivery status = %v", first["status"])
	}

	// A repeat delivery for the SAME host+service must fold into the existing
	// incident, not mint a second one. That is the idempotency the script owns
	// (rela offers no dedup key here) and the quiet failure this pins.
	rec = postRequestAction(t, app, "icinga-alert", alert, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("second delivery: expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	second := decodeIncidentResult(t, rec.Body.String())
	if second["status"] != "appended" {
		t.Fatalf("second delivery status = %v", second["status"])
	}
	if second["incident"] != first["incident"] {
		t.Fatalf("a repeat alert created a second incident: %v then %v",
			first["incident"], second["incident"])
	}

	// A payload with no host is a 422 — well-formed, but it cannot make a valid
	// entity, so a sender that distinguishes the two stops retrying.
	rec = postRequestAction(t, app, "icinga-alert", `{"service":"http"}`, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("hostless alert: expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	// A non-JSON payload is a 400 the script itself chose, reachable only
	// because rela.request.body is nil while rela.request.raw is not.
	rec = postRequestAction(t, app, "icinga-alert", `<xml/>`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-JSON alert: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// withIncidentType adds the entity type the example writes to. The shared V1
// fixture has only ticket/feature, and widening it for one example would make
// every unrelated test carry the type.
func withIncidentType(app *App) {
	meta := app.State().Meta
	meta.Entities["incident"] = metamodel.EntityDef{
		Label:    "Incident",
		IDPrefix: "INC-",
		Properties: map[string]metamodel.PropertyDef{
			"title":     {Type: "string", Required: true},
			"alert_key": {Type: "string"},
			"status":    {Type: "string"},
		},
		PropertyOrder: []string{"title", "alert_key", "status"},
	}
}

// readExampleAction loads a script from the repo's examples/ directory. The
// test runs from internal/dataentry, so examples/ is two levels up.
func readExampleAction(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "examples", name))
	if err != nil {
		t.Fatalf("read examples/%s: %v", name, err)
	}
	return string(b)
}

// decodeIncidentResult parses the example's JSON answer.
func decodeIncidentResult(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&out); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return out
}
