package dataentry

import (
	"context"
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

// TestAction_HiddenFieldRedacted covers the FIELD-level half of the gate.
//
// A row-level "may you read this entity" check is not enough: the script
// receives the whole entity via EntityToTable, which redacts nothing. A
// principal permitted the ROW but denied a `visible:`-gated FIELD would
// otherwise read that field's value out of script scope — the action's own
// response is the exfiltration path.
//
// visibility.ScriptReader.GetEntity filters the entity before it is handed
// over, so the hidden property is absent rather than merely unprinted.
func TestAction_HiddenFieldRedacted(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"peek.lua": `return { message = tostring(entity.properties.status) }`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-MIX", Type: "ticket",
		Properties: map[string]any{"title": "visible", "status": "SUPERSECRET"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{"peek": {Script: "peek.lua"}}

	// alice may read tickets; `status` is hidden at the field level. Redaction
	// resolves through app.fieldResolver (the affordance seam), not the raw
	// policy — same wiring the _views redaction tests use.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"status": false},
	}}

	rec := postAction(t, app, d, aliceCtx(), "peek", `{"entity_id":"TKT-MIX"}`)
	if strings.Contains(rec.Body.String(), "SUPERSECRET") {
		t.Errorf("LEAK: a visible:-hidden field value reached script scope: %s", rec.Body)
	}
}

// TestAction_DeniedAndAbsentAreIndistinguishable pins the oracle-free
// contract: a denied id and a nonexistent id must produce the SAME response.
//
// Getting this wrong turns the endpoint into a binary existence oracle over
// the whole id space — the exact invariant entityNotFoundTitle exists to keep
// on the entity routes (RR-NGMI). An earlier draft of this fix denied with a
// 404 while an absent id returned 200, which is that oracle.
func TestAction_DeniedAndAbsentAreIndistinguishable(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `
			local out = "no-entity"
			if entity ~= nil then out = entity.properties.title or "no-title" end
			return { message = out }
		`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-EXISTS", Type: "ticket", Properties: map[string]any{"title": "classified"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{"echo": {Script: "echo.lua"}}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"outsider": {}},
		Assignments: map[string]string{"mallory": "outsider"},
	}, app.store)
	app.acl = d

	denied := postAction(t, app, d, principalCtx("mallory"), "echo", `{"entity_id":"TKT-EXISTS"}`)
	absent := postAction(t, app, d, principalCtx("mallory"), "echo", `{"entity_id":"TKT-NOPE"}`)

	if denied.Code != absent.Code {
		t.Errorf("existence oracle: denied id → %d, absent id → %d (must match)",
			denied.Code, absent.Code)
	}
	if denied.Body.String() != absent.Body.String() {
		t.Errorf("existence oracle: denied and absent bodies differ:\n denied=%s\n absent=%s",
			denied.Body, absent.Body)
	}
	if strings.Contains(denied.Body.String(), "classified") {
		t.Errorf("LEAK: denied entity reached the script: %s", denied.Body)
	}
}

// TestAction_RunsWhenEntityDeniedButUnused covers the availability half.
//
// entity_id is an optional PARAMETER, not the resource. An action whose script
// never touches `entity` must still run when the caller supplies an id they
// cannot read — the SPA sends an id for every selected list row, so refusing
// the request would fail a bulk operation the caller is entitled to perform
// because one row in the selection happened to be invisible to them.
func TestAction_RunsWhenEntityDeniedButUnused(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"sideeffect.lua": `return { message = "side-effect-done" }`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-HIDDEN", Type: "ticket", Properties: map[string]any{"title": "nope"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{
		"sideeffect": {Script: "sideeffect.lua"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"outsider": {}},
		Assignments: map[string]string{"mallory": "outsider"},
	}, app.store)
	app.acl = d

	rec := postAction(t, app, d, principalCtx("mallory"), "sideeffect", `{"entity_id":"TKT-HIDDEN"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("an unreadable optional parameter must not fail the action: got %d; body=%s",
			rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "side-effect-done") {
		t.Errorf("action did not run: %s", rec.Body)
	}
}

// TestAction_EntityTypeIsIgnored pins that entity_type cannot influence the
// outcome at all. TestAction_EntityIDCrossTypeEscalation proves a MISMATCHED
// claim denies; this proves the field is simply unused — a gate that ORed the
// stored and claimed types would pass that test and fail this one.
func TestAction_EntityTypeIsIgnored(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `
			local out = "no-entity"
			if entity ~= nil then out = entity.properties.title or "no-title" end
			return { message = out }
		`,
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-OK", Type: "ticket", Properties: map[string]any{"title": "readable"},
	})
	app.State().Cfg.Actions = map[string]dataentryconfig.Action{"echo": {Script: "echo.lua"}}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// Same id, three different claims — including a nonsense one. All must
	// resolve identically, because only the stored type is consulted.
	var bodies []string
	for _, claim := range []string{`"ticket"`, `"feature"`, `"not-a-type"`} {
		rec := postAction(t, app, d, aliceCtx(), "echo",
			`{"entity_id":"TKT-OK","entity_type":`+claim+`}`)
		bodies = append(bodies, rec.Body.String())
	}
	for i, b := range bodies {
		if !strings.Contains(b, "readable") {
			t.Errorf("claim %d changed the outcome; entity_type must be ignored: %s", i, b)
		}
		if b != bodies[0] {
			t.Errorf("claim %d produced a different response:\n %s\n %s", i, bodies[0], b)
		}
	}
}

// postAction is the shared driver for these tests.
func postAction(t *testing.T, app *App, d *acl.Declarative,
	ctx context.Context, actionID, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/"+actionID,
		strings.NewReader(body)).WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	callAction(app, req, rec)
	return rec
}
