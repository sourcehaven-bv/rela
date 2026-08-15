package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestAction_EntityIDRespectsReadGate pins that the optional `entity_id` in an
// action request cannot be used to read an entity the principal may not see.
//
// handleV1Action resolves that id and hands the resulting entity to the script
// as the global `entity`. Resolving it through the RAW store would make the
// action endpoint a read-side ACL bypass: any caller who may POST an action
// could name any id and have its full properties — including `visible:`-hidden
// fields — placed in script scope, then echoed back via the action's response.
//
// The gate must therefore run on the caller's behalf, not the script's.
func TestAction_EntityIDRespectsReadGate(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `
			local out = "no-entity"
			if entity ~= nil then out = entity.properties.title or "no-title" end
			return { message = out }
		`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-SECRET", Type: "ticket",
		Properties: map[string]any{"title": "classified"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{
		"echo": {Script: "echo.lua"},
	}

	// mallory holds no role at all, so she may not read tickets.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	body := `{"entity_id":"TKT-SECRET","entity_type":"ticket"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/echo",
		strings.NewReader(body)).WithContext(gateCtxFor(principalCtx("mallory"), t, d))
	rec := httptest.NewRecorder()
	callAction(app, req, rec)

	if strings.Contains(rec.Body.String(), "classified") {
		t.Errorf("LEAK: entity_id resolved through the raw store, exposing a "+
			"hidden entity's properties to the script: %s", rec.Body)
	}
}

// TestAction_EntityIDCrossTypeEscalation pins that the gate consults the
// STORED entity type, not the caller-supplied entity_type.
//
// Authorizing against a claimed type is a cross-type escalation: name a type
// you may read (feature), an id of a type you may not (ticket), and an
// AllowAll verdict on the claim would grant the id. BUG-ZWTDH9 was this same
// defect on the sync channel.
func TestAction_EntityIDCrossTypeEscalation(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `
			local out = "no-entity"
			if entity ~= nil then out = entity.properties.title or "no-title" end
			return { message = out }
		`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-SECRET", Type: "ticket",
		Properties: map[string]any{"title": "classified"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{"echo": {Script: "echo.lua"}}

	// bob may read features, never tickets.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"featreader": {Read: []string{"feature"}}},
		Assignments: map[string]string{"bob": "featreader"},
	}, app.store)
	app.acl = d

	// He claims the ticket is a feature.
	body := `{"entity_id":"TKT-SECRET","entity_type":"feature"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/echo",
		strings.NewReader(body)).WithContext(gateCtxFor(principalCtx("bob"), t, d))
	rec := httptest.NewRecorder()
	callAction(app, req, rec)

	if strings.Contains(rec.Body.String(), "classified") {
		t.Errorf("ESCALATION: gate honored the caller-supplied entity_type "+
			"instead of the stored one: %s", rec.Body)
	}
}

// TestAction_EntityIDPermittedStillWorks is the other direction: the gate must
// not break the feature it protects. A principal who MAY read the entity still
// gets it in script scope.
func TestAction_EntityIDPermittedStillWorks(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `
			local out = "no-entity"
			if entity ~= nil then out = entity.properties.title or "no-title" end
			return { message = out }
		`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-OK", Type: "ticket",
		Properties: map[string]any{"title": "readable"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{"echo": {Script: "echo.lua"}}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	body := `{"entity_id":"TKT-OK","entity_type":"ticket"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/echo",
		strings.NewReader(body)).WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	callAction(app, req, rec)

	if !strings.Contains(rec.Body.String(), "readable") {
		t.Errorf("gating broke the permitted path; alice may read TKT-OK: %s", rec.Body)
	}
}
