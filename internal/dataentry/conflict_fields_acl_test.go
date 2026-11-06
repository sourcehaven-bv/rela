package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestV1ConflictResolve_FieldGateRealPolicy is TestV1ConflictResolve_FieldWriteGate
// and its relation half against a compiled acl.yaml policy instead of stubbed
// verdicts (BUG-197XPAU). Each role may read and update tickets; the
// `fields:`, `visible:` and relation `fields:` grants decide whether the
// field the sides disagree on may be resolved.
func TestV1ConflictResolve_FieldGateRealPolicy(t *testing.T) {
	ticketGrant := func(fields, visible []acl.FieldGrant, relFields []acl.FieldGrant) acl.RoleDef {
		role := acl.RoleDef{
			Read: []string{"ticket"}, Update: []string{"ticket"}, Permissions: []string{acl.PermGitSync},
			Relations: map[string][]acl.RelationGrant{"ticket": {{Relation: "blocks", Fields: relFields}}},
		}
		if fields != nil {
			role.Fields = map[string][]acl.FieldGrant{"ticket": fields}
		}
		if visible != nil {
			role.Visible = map[string][]acl.FieldGrant{"ticket": visible}
		}
		return role
	}
	tests := []struct {
		name     string
		role     acl.RoleDef
		path     string
		file     string
		body     string
		wantCode int
	}{
		{
			name:     "title outside the fields grant",
			role:     ticketGrant([]acl.FieldGrant{{Field: "status"}}, nil, nil),
			path:     "entities/ticket/TKT-001.md",
			file:     conflictedTicket,
			body:     `{"path":"entities/ticket/TKT-001.md","property_choices":{"title":"theirs"}}`,
			wantCode: http.StatusForbidden,
		},
		{
			name:     "title hidden by visible",
			role:     ticketGrant(nil, []acl.FieldGrant{{Field: "status"}}, nil),
			path:     "entities/ticket/TKT-001.md",
			file:     conflictedTicket,
			body:     `{"path":"entities/ticket/TKT-001.md","property_choices":{"title":"ours"}}`,
			wantCode: http.StatusForbidden,
		},
		{
			name:     "title inside the fields grant",
			role:     ticketGrant([]acl.FieldGrant{{Field: "title"}}, nil, nil),
			path:     "entities/ticket/TKT-001.md",
			file:     conflictedTicket,
			body:     `{"path":"entities/ticket/TKT-001.md","property_choices":{"title":"theirs"}}`,
			wantCode: http.StatusOK,
		},
		{
			name:     "relation meta whose fields grant does not hold",
			role:     ticketGrant(nil, nil, []acl.FieldGrant{{Field: "note", When: "entity.title == 'LOCKED'"}}),
			path:     "relations/TKT-001--blocks--TKT-002.md",
			file:     conflictedRelation,
			body:     `{"path":"relations/TKT-001--blocks--TKT-002.md","property_choices":{"note":"theirs"}}`,
			wantCode: http.StatusForbidden,
		},
		{
			// BUG-O8QP7GX: a declared meta field no role names is outside
			// the allowlist, as an entity property is.
			name:     "relation meta no fields grant names",
			role:     ticketGrant(nil, nil, []acl.FieldGrant{{Field: "weight"}}),
			path:     "relations/TKT-001--blocks--TKT-002.md",
			file:     conflictedRelation,
			body:     `{"path":"relations/TKT-001--blocks--TKT-002.md","property_choices":{"note":"theirs"}}`,
			wantCode: http.StatusForbidden,
		},
		{
			// An undeclared key no grant names comes from the file, so it does
			// not block the resolve, as for an entity.
			name:     "undeclared relation meta no grant names",
			role:     ticketGrant(nil, nil, []acl.FieldGrant{{Field: "weight"}}),
			path:     "relations/TKT-001--blocks--TKT-002.md",
			file:     strings.ReplaceAll(conflictedRelation, "note:", "extra:"),
			body:     `{"path":"relations/TKT-001--blocks--TKT-002.md","property_choices":{"extra":"theirs"}}`,
			wantCode: http.StatusOK,
		},
		{
			name:     "relation meta inside the relation fields grant",
			role:     ticketGrant(nil, nil, []acl.FieldGrant{{Field: "note"}}),
			path:     "relations/TKT-001--blocks--TKT-002.md",
			file:     conflictedRelation,
			body:     `{"path":"relations/TKT-001--blocks--TKT-002.md","property_choices":{"note":"theirs"}}`,
			wantCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, root, d, sink := newConflictACLApp(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"resolver": tc.role},
				Assignments: map[string]string{"alice": "resolver"},
			}, func(app *App) {
				// Declare the meta fields, so the grants judge declared fields.
				meta := app.State().Meta
				blocks := meta.Relations["blocks"]
				blocks.Properties = map[string]metamodel.PropertyDef{"note": {Type: "string"}, "weight": {Type: "string"}}
				meta.Relations["blocks"] = blocks
				seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
				seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
			})
			resolver, err := affordances.New(app.State().Meta, storeRelationLookup{st: app.store}, d)
			if err != nil {
				t.Fatalf("affordances.New: %v", err)
			}
			setTestResolver(t, app, &policyResolver{inner: resolver})
			path := writeProjectFile(t, root, tc.path, tc.file)

			rec := httptest.NewRecorder()
			app.write.handleV1ConflictResolve(rec, asAlice(t, d, httptest.NewRequest(http.MethodPost,
				"/api/v1/_conflicts/resolve", strings.NewReader(tc.body))))

			if rec.Code != tc.wantCode {
				t.Fatalf("resolve = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body)
			}
			got, _ := os.ReadFile(path)
			if tc.wantCode == http.StatusOK {
				if strings.Contains(string(got), "<<<<<<<") {
					t.Errorf("allowed resolve left the markers:\n%s", got)
				}
				return
			}
			if string(got) != tc.file {
				t.Error("refused resolve rewrote the file")
			}
			for _, r := range sink.Records() {
				if r.Op == audit.OpUpdateEntity || r.Op == audit.OpUpdateRelation {
					t.Errorf("refused resolve audited a write: %+v", r)
				}
			}
		})
	}
}

// TestV1RelationMeta_FieldsGrantIsAnAllowlist pins BUG-O8QP7GX on the
// relation meta write paths against a compiled policy: once a role declares
// meta `fields:` for a relation type, a meta key no role grants is refused,
// declared or not. A key sent back with its stored value is no write and
// needs no grant. The role union is pinned in internal/affordances
// (TestRelationMeta_FieldsGrantIsAnAllowlist).
func TestV1RelationMeta_FieldsGrantIsAnAllowlist(t *testing.T) {
	clerk := acl.RoleDef{
		Read: []string{"ticket"}, Update: []string{"ticket"}, Create: []string{"ticket"},
		Relations: map[string][]acl.RelationGrant{"ticket": {{
			Relation: "blocks", Fields: []acl.FieldGrant{{Field: "weight"}},
		}}},
	}
	const (
		patchEdge   = "PATCH relation"
		unified     = "unified PATCH"
		createEdge  = "POST relation"
		storedNote  = "old"
		storedExtra = "kept"
	)
	tests := []struct {
		name     string
		path     string
		meta     string
		wantCode int
	}{
		{"granted field", patchEdge, `{"weight":"3"}`, http.StatusOK},
		{"declared field no role names", patchEdge, `{"note":"x"}`, http.StatusForbidden},
		{"undeclared field", patchEdge, `{"extra":"x"}`, http.StatusForbidden},
		{"unchanged ungranted fields sent back", unified,
			`{"weight":"4","note":"` + storedNote + `","extra":"` + storedExtra + `"}`, http.StatusOK},
		{"changed declared field", unified, `{"weight":"4","note":"x"}`, http.StatusForbidden},
		{"changed undeclared field", unified, `{"extra":"x"}`, http.StatusForbidden},
		{"create with granted field", createEdge, `{"weight":"1"}`, http.StatusCreated},
		{"create with undeclared field", createEdge, `{"extra":"x"}`, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.path+": "+tc.name, func(t *testing.T) {
			app, _, d, _ := newConflictACLApp(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"clerk": clerk},
				Assignments: map[string]string{"alice": "clerk"},
			}, func(app *App) {
				meta := app.State().Meta
				blocks := meta.Relations["blocks"]
				blocks.Properties = map[string]metamodel.PropertyDef{"note": {Type: "string"}, "weight": {Type: "string"}}
				meta.Relations["blocks"] = blocks
				for _, id := range []string{"TKT-001", "TKT-002", "TKT-003"} {
					seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
				}
				if _, err := app.store.CreateRelation(context.Background(),
					entity.RelationKey{From: "TKT-001", Type: "blocks", To: "TKT-002"},
					&store.RelationData{Properties: map[string]any{"note": storedNote, "extra": storedExtra}},
				); err != nil {
					t.Fatalf("CreateRelation: %v", err)
				}
			})
			resolver, err := affordances.New(app.State().Meta, storeRelationLookup{st: app.store}, d)
			if err != nil {
				t.Fatalf("affordances.New: %v", err)
			}
			setTestResolver(t, app, &policyResolver{inner: resolver})

			rec := httptest.NewRecorder()
			switch tc.path {
			case patchEdge:
				app.write.handleV1UpdateRelation(rec, asAlice(t, d, httptest.NewRequest(http.MethodPatch,
					"/api/v1/tickets/TKT-001/relations/blocks/TKT-002",
					strings.NewReader(`{"meta":`+tc.meta+`}`))), "ticket", "TKT-001", "blocks", "TKT-002")
			case unified:
				app.write.handleV1UpdateEntity(rec, asAlice(t, d, httptest.NewRequest(http.MethodPatch,
					"/api/v1/tickets/TKT-001", strings.NewReader(
						`{"relations":{"blocks":{"data":[{"type":"ticket","id":"TKT-002","meta":`+tc.meta+`}]}}}`))),
					"ticket", "tickets", "TKT-001")
			case createEdge:
				app.write.handleV1CreateRelation(rec, asAlice(t, d, httptest.NewRequest(http.MethodPost,
					"/api/v1/tickets/TKT-001/relations/blocks",
					strings.NewReader(`{"id":"TKT-003","meta":`+tc.meta+`}`))), "ticket", "TKT-001", "blocks")
			}

			if rec.Code != tc.wantCode {
				t.Fatalf("%s = %d, want %d: %s", tc.path, rec.Code, tc.wantCode, rec.Body)
			}
			if tc.wantCode == http.StatusForbidden &&
				!strings.Contains(rec.Body.String(), "relation-affordance:meta-read-only:blocks.") {

				t.Errorf("body must name the meta-read-only rule: %s", rec.Body)
			}
		})
	}
}
