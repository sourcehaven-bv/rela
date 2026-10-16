package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
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
	setTestACL(t, app, d)
	setTestResolver(t, app, &policyResolver{inner: resolver})
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

// TestACLList_BatchedIncludesKeepGates pins the read gates on the batched
// list include path (TASK-550RD). The page's include block is resolved in
// one batch over every row, so a gate verdict must still hold per entity:
// a type the principal cannot read never enters `included`, and a hidden
// field of an included entity is redacted, for every row on the page.
func TestACLList_BatchedIncludesKeepGates(t *testing.T) {
	const aclYAML = `
roles:
  viewer:
    read: [ticket]
    visible:
      ticket:
        - field: title
  admin:
    read: ["*"]
assignments:
  alice: viewer
  root: admin
`
	app, d := buildFieldPolicyApp(t, aclYAML)
	for _, id := range []string{"TKT-001", "TKT-002", "TKT-003"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": "title " + id, "status": "hidden-status-" + id}})
	}
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "secret feature"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-002", Type: "feature", Properties: map[string]any{"title": "other feature"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))
	seedRelation(app, entity.NewRelation("TKT-003", "implements", "FEAT-002"))
	seedRelation(app, entity.NewRelation("TKT-002", "blocks", "TKT-001"))
	seedRelation(app, entity.NewRelation("TKT-003", "blocks", "TKT-002"))

	type listBody struct {
		Included map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"included"`
	}
	decode := func(t *testing.T, raw []byte) listBody {
		t.Helper()
		var b listBody
		if err := json.Unmarshal(raw, &b); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return b
	}

	for _, include := range []string{"*", "implements,blocks", "blocks.implements"} {
		t.Run("viewer include="+include, func(t *testing.T) {
			_, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "include="+url.QueryEscape(include))
			if rec.Code != http.StatusOK {
				t.Fatalf("list: %d %s", rec.Code, rec.Body)
			}
			body := rec.Body.String()
			for _, leak := range []string{"secret feature", "other feature", "hidden-status-"} {
				if strings.Contains(body, leak) {
					t.Errorf("batched include leaked %q: %s", leak, body)
				}
			}
			b := decode(t, rec.Body.Bytes())
			for id, e := range b.Included {
				if strings.HasPrefix(id, "FEAT-") {
					t.Errorf("hidden %s served in included", id)
				}
				if _, ok := e.Properties["status"]; ok {
					t.Errorf("included %s carries hidden status", id)
				}
			}
			if _, ok := b.Included["TKT-001"]; !ok {
				t.Errorf("readable TKT-001 missing from included: %s", body)
			}
		})
	}

	// Control: a principal who may read everything gets both features and
	// the statuses, so the assertions above are not vacuous.
	rootCtx := principal.With(context.Background(), principal.Principal{User: "root", Tool: principal.ToolDataEntry})
	_, rec := listEntitiesAs(rootCtx, t, app, d, "ticket", "tickets", "include=*")
	if rec.Code != http.StatusOK {
		t.Fatalf("root list: %d %s", rec.Code, rec.Body)
	}
	b := decode(t, rec.Body.Bytes())
	for _, id := range []string{"FEAT-001", "FEAT-002", "TKT-001", "TKT-002", "TKT-003"} {
		if _, ok := b.Included[id]; !ok {
			t.Errorf("root: %s missing from included: %s", id, rec.Body)
		}
	}
	if s, _ := b.Included["TKT-001"].Properties["status"].(string); s != "hidden-status-TKT-001" {
		t.Errorf("root: TKT-001 status = %q, want it visible", s)
	}
}
