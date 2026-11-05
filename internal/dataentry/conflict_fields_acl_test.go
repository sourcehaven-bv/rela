package dataentry

import (
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
