package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestRelationPosition_FieldsGrantRealPolicy pins the position PATCH against
// a compiled policy with a relation meta `fields:` grant (BUG-FQZ5Q1L). The
// order key is a position, not content: a grant that does not name it leaves
// it writable, as for an explicit `meta: {_order_out: n}`
// (relationMetaFields). A grant that names it decides the move.
func TestRelationPosition_FieldsGrantRealPolicy(t *testing.T) {
	order := metamodel.OrderPropertyOut
	tests := []struct {
		name     string
		fields   []acl.FieldGrant
		wantCode int
	}{
		{"grant names another field only", []acl.FieldGrant{{Field: "weight"}}, http.StatusNoContent},
		{"grant names the order field", []acl.FieldGrant{{Field: "weight"}, {Field: order}}, http.StatusNoContent},
		{
			"grant names the order field under a failing when",
			[]acl.FieldGrant{{Field: "weight"}, {Field: order, When: "entity.title == 'LOCKED'"}},
			http.StatusForbidden,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clerk := acl.RoleDef{
				Read: []string{"ticket"}, Update: []string{"ticket"},
				Relations: map[string][]acl.RelationGrant{"ticket": {{Relation: "blocks", Fields: tc.fields}}},
			}
			keys := []entity.RelationKey{
				{From: "TKT-001", Type: "blocks", To: "TKT-002"},
				{From: "TKT-001", Type: "blocks", To: "TKT-003"},
			}
			app, _, d, _ := newConflictACLApp(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"clerk": clerk},
				Assignments: map[string]string{"alice": "clerk"},
			}, func(app *App) {
				meta := app.State().Meta
				blocks := meta.Relations["blocks"]
				blocks.Properties = map[string]metamodel.PropertyDef{"weight": {Type: "string"}}
				blocks.Orderable = metamodel.OrderableOutgoing
				meta.Relations["blocks"] = blocks
				for _, id := range []string{"TKT-001", "TKT-002", "TKT-003"} {
					seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
				}
				for i, key := range keys {
					if _, err := app.store.CreateRelation(context.Background(), key,
						&store.RelationData{Properties: map[string]any{order: float64(i + 1)}}); err != nil {
						t.Fatalf("CreateRelation: %v", err)
					}
				}
			})
			resolver, err := affordances.New(app.State().Meta, storeRelationLookup{st: app.store}, d)
			if err != nil {
				t.Fatalf("affordances.New: %v", err)
			}
			setTestResolver(t, app, &policyResolver{inner: resolver})

			rec := httptest.NewRecorder()
			app.write.handleV1UpdateRelation(rec, asAlice(t, d, httptest.NewRequest(http.MethodPatch,
				"/api/v1/tickets/TKT-001/relations/blocks/TKT-002",
				strings.NewReader(`{"position":{"step":1}}`))), "ticket", "TKT-001", "blocks", "TKT-002")
			if rec.Code != tc.wantCode {
				t.Fatalf("move = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body)
			}

			moved, err := app.store.GetRelation(context.Background(), keys[0])
			if err != nil {
				t.Fatalf("GetRelation: %v", err)
			}
			if tc.wantCode == http.StatusForbidden {
				if !strings.Contains(rec.Body.String(), "meta-read-only:blocks."+order) {
					t.Errorf("body must name the meta-read-only rule: %s", rec.Body)
				}
				if moved.Properties[order] != 1.0 {
					t.Errorf("refused move wrote %s = %v", order, moved.Properties[order])
				}
				return
			}
			if moved.Properties[order] == 1.0 {
				t.Errorf("allowed move left %s at 1", order)
			}
		})
	}
}
