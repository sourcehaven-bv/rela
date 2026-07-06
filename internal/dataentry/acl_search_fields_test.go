package dataentry

import (
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// buildFieldPolicyApp wires ONE acl.Declarative as both the read gate (so a
// granted type is entity-level visible) and the field resolver (so a `visible:`
// block redacts a property). This is the combination the search-oracle fix
// needs: the entity is readable, but one of its properties is hidden.
func buildFieldPolicyApp(t *testing.T, aclYAML string) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)

	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(aclYAML), &policy); err != nil {
		t.Fatalf("unmarshal acl.yaml: %v", err)
	}
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(app.store), app.store)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(app.Meta(), storeRelationLookup{st: app.store}, d)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	app.acl = d
	app.fieldResolver = &policyResolver{inner: resolver}
	return app, d
}

// TestACLSearch_HiddenFieldOracleClosed is the end-to-end payoff for
// TKT-GGQ0JT: a principal who may READ a ticket but whose role hides the
// ticket's `status` property must NOT be able to confirm a hidden status value
// by searching for it. A search whose only match is the hidden property returns
// nothing; a search matching a visible field still returns the ticket (with the
// hidden property redacted from the body).
//
// `visible: {ticket: [title]}` is closed-world: only `title` is visible;
// `status` is hidden. The status value "zeta777status" appears in no other
// field, so a search for it can match only via the hidden property.
func TestACLSearch_HiddenFieldOracleClosed(t *testing.T) {
	const aclYAML = `
roles:
  viewer:
    read: [ticket]
    visible:
      ticket:
        - field: title
assignments:
  alice: viewer
`
	app, d := buildFieldPolicyApp(t, aclYAML)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "alpha rocket", "status": "zeta777status"},
	})

	// Oracle probe: searching the hidden status value returns nothing.
	resp, rec := searchAs(aliceCtx(), t, app, d, "zeta777status")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search zeta777status: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 || resp.Meta.Total != 0 {
		t.Errorf("hidden-field oracle: got %d hits (total %d), want 0 — search leaked a hidden property",
			len(resp.Data), resp.Meta.Total)
	}
	if strings.Contains(rec.Body.String(), "zeta777status") {
		t.Errorf("hidden status value leaked into the search body")
	}

	// Control: searching a visible field still returns the ticket, status redacted.
	resp, rec = searchAs(aliceCtx(), t, app, d, "alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search alpha: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Fatalf("visible-field search: got %d hits, want the ticket", len(resp.Data))
	}
	if strings.Contains(rec.Body.String(), "zeta777status") {
		t.Errorf("hidden `status` value leaked in a visible-field hit's body")
	}
}

// TestACLSearch_HiddenFieldVisibleToPrivilegedRole is the negative control: a
// role that does NOT declare a `visible:` block for ticket leaves all fields
// visible, so the same "zeta777" search DOES return the ticket. This proves the
// drop in the previous test is caused by the visibility policy, not a bug that
// swallows property matches wholesale.
func TestACLSearch_HiddenFieldVisibleToPrivilegedRole(t *testing.T) {
	const aclYAML = `
roles:
  admin:
    read: ["*"]
assignments:
  alice: admin
`
	app, d := buildFieldPolicyApp(t, aclYAML)
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "alpha rocket", "status": "zeta777status"},
	})

	resp, rec := searchAs(aliceCtx(), t, app, d, "zeta777status")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search zeta777status: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Errorf("no visible: block → property visible → search should return the ticket; got %d hits",
			len(resp.Data))
	}
}
