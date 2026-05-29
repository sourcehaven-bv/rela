package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"

	"gopkg.in/yaml.v3"
)

// buildPolicyApp wires a real affordances.Resolver (compiled from the
// given acl.yaml source) behind the dataentry policyResolver, sharing
// the App's store as the relation lookup. This exercises the full
// production chain end-to-end: acl.yaml → affordances.New →
// policyResolver → wire shape / 403 / audit.
func buildPolicyApp(t *testing.T, aclYAML string, sink audit.Audit) *App {
	t.Helper()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)

	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(aclYAML), &policy); err != nil {
		t.Fatalf("unmarshal acl.yaml: %v", err)
	}
	resolver, err := affordances.New(&policy, app.Meta(), storeRelationLookup{st: app.store})
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	app.fieldResolver = &policyResolver{inner: resolver}
	return app
}

func patchAs(t *testing.T, app *App, user, id, body string) (status int, respBody string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/"+id, strings.NewReader(body))
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", id)
	return rec.Code, rec.Body.String()
}

func getAs(t *testing.T, app *App, user, id string) V1Entity {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/"+id, http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: got %d, want 200; body=%s", id, rec.Code, rec.Body.String())
	}
	var e V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&e); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	return e
}

// AC3 end-to-end: a field grant whose predicate evaluates false yields
// _fields.status.writable=false on GET AND a 403 on the PATCH, with
// the TKT-G7N5 wire rule_id (DR-C5 — not a new rule_kind).
func TestPolicyResolver_FieldPredicate_WireAndWrite(t *testing.T) {
	const aclYAML = `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.title == 'editable'"
assignments:
  alice: triager
`
	app := buildPolicyApp(t, aclYAML, nil)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "locked", "status": "open"},
	})

	// GET: status is denied (title != "editable") → writable=false.
	got := getAs(t, app, "alice", "TKT-001")
	if got.FieldAffordances == nil {
		t.Fatal("FieldAffordances nil")
	}
	fa := *got.FieldAffordances
	if w := fa["status"].Writable; w == nil || *w {
		t.Errorf("status.writable: got %v, want *false", w)
	}

	// PATCH status → 403 with the TKT-G7N5 wire rule_id.
	code, body := patchAs(t, app, "alice", "TKT-001", `{"properties":{"status":"closed"}}`)
	if code != http.StatusForbidden {
		t.Fatalf("PATCH: got %d, want 403; body=%s", code, body)
	}
	if !strings.Contains(body, `"field-affordance:read-only:status"`) {
		t.Errorf("body must carry the TKT-G7N5 rule_id, got: %s", body)
	}
	// The external body must NOT leak the role/predicate attribution.
	if strings.Contains(body, "role=triager") || strings.Contains(body, "attribution") {
		t.Errorf("wire body leaked attribution: %s", body)
	}
}

// DR-C5 two-channel: the audit Summary carries the role attribution the
// wire body withholds.
func TestPolicyResolver_AuditCarriesAttribution(t *testing.T) {
	const aclYAML = `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.title == 'editable'"
assignments:
  alice: triager
`
	sink := audit.NewMemory()
	app := buildPolicyApp(t, aclYAML, sink)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "locked", "status": "open"},
	})

	// PATCH directly, with NO prior GET — attribution rides on the
	// write-path verdict, not a GET-populated side table (RR-1DRR).
	code, _ := patchAs(t, app, "alice", "TKT-001", `{"properties":{"status":"closed"}}`)
	if code != http.StatusForbidden {
		t.Fatalf("PATCH: got %d, want 403", code)
	}
	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("audit records: got %d, want 1", len(records))
	}
	sum := records[0].Summary
	if !strings.Contains(sum, "rule_id=field-affordance:read-only:status") {
		t.Errorf("audit summary missing wire rule_id: %q", sum)
	}
	if !strings.Contains(sum, "attribution=") || !strings.Contains(sum, "role=triager") {
		t.Errorf("audit summary missing role attribution: %q", sum)
	}
}

// AC5 end-to-end: a relation grant with create:false yields
// _relations.depends_on.creatable=false AND a 403 on the actual
// relation-create POST (S5: drive the write, not just the wire shape).
func TestPolicyResolver_RelationCreate_WireAndWrite(t *testing.T) {
	const aclYAML = `
roles:
  triager:
    relations:
      ticket:
        - relation: depends_on
          create: false
          remove: true
assignments:
  alice: triager
`
	app := buildPolicyApp(t, aclYAML, nil)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID:         "TKT-002",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "y", "status": "open"},
	})

	// Wire shape: creatable=false (sparse).
	got := getAs(t, app, "alice", "TKT-001")
	if got.RelationAffordances == nil {
		t.Fatal("RelationAffordances nil")
	}
	dep, ok := (*got.RelationAffordances)["depends_on"]
	if !ok {
		t.Fatalf("depends_on affordance missing; got %+v", *got.RelationAffordances)
	}
	if dep.Creatable == nil || *dep.Creatable {
		t.Errorf("depends_on.creatable: got %v, want *false", dep.Creatable)
	}
	if dep.Removable != nil && !*dep.Removable {
		t.Errorf("depends_on.removable: got *false, want allowed (nil or *true)")
	}

	// Write: POST .../relations/depends_on → 403 with the create rule_id.
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/tickets/TKT-001/relations/depends_on",
		strings.NewReader(`{"id":"TKT-002"}`))
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1CreateRelation(rec, req, "ticket", "TKT-001", "depends_on")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create POST: got %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relation-affordance:not-creatable:depends_on") {
		t.Errorf("body must carry the not-creatable rule_id, got: %s", rec.Body.String())
	}
}

// AC2 end-to-end: a policy with no affordance blocks produces no
// _fields / _relations deviations — byte-identical to the Nop resolver.
func TestPolicyResolver_NoAffordanceBlocks_PermissiveWire(t *testing.T) {
	// HasAffordanceGrants is false here, so ResolverFromProfile would
	// pick Nop. We assert the resolver path directly: a write-only
	// policy run through affordances.New yields empty verdicts.
	const aclYAML = `
roles:
  admin: { write: ["*"] }
assignments:
  alice: admin
`
	app := buildPolicyApp(t, aclYAML, nil)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "x", "status": "open"},
	})

	got := getAs(t, app, "alice", "TKT-001")
	if got.FieldAffordances != nil && len(*got.FieldAffordances) != 0 {
		t.Errorf("expected no field affordances, got %+v", *got.FieldAffordances)
	}
	if got.RelationAffordances != nil && len(*got.RelationAffordances) != 0 {
		t.Errorf("expected no relation affordances, got %+v", *got.RelationAffordances)
	}
}
