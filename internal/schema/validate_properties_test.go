package schema_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestValidateEntityProperties(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:      "Ticket",
				IDPrefixes: []string{"TKT-"},
				Properties: map[string]metamodel.PropertyDef{
					"status": {
						Type:     "enum",
						Required: true,
						Values:   []string{"open", "closed"},
					},
				},
			},
		},
	}

	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"status": "open"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"status": "invalid"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed entity %s: %v", e.ID, err)
		}
	}

	errs := schema.ValidateEntityProperties(ctx, st, meta)
	if len(errs) != 1 {
		t.Fatalf("got %d entities with errors, want 1", len(errs))
	}
	if errs[0].EntityID != "TKT-002" {
		t.Errorf("error entity = %s, want TKT-002", errs[0].EntityID)
	}
}

func TestValidateRelationProperties(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefixes: []string{"TKT-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"blocks": {
				Label: "blocks",
				From:  []string{"ticket"},
				To:    []string{"ticket"},
				Properties: map[string]metamodel.PropertyDef{
					"since": {Type: "date", Required: true},
				},
			},
		},
	}

	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket"},
		{ID: "TKT-002", Type: "ticket"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed entity %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "TKT-001", "blocks", "TKT-002", &store.RelationData{
		Properties: map[string]interface{}{"since": "not-a-date"},
	}); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	errs := schema.ValidateRelationProperties(ctx, st, meta)
	if len(errs) != 1 {
		t.Fatalf("got %d relations with errors, want 1", len(errs))
	}
	if errs[0].RelationType != "blocks" {
		t.Errorf("error relation type = %s, want blocks", errs[0].RelationType)
	}
}

func TestValidateProperties_NilInputs(t *testing.T) {
	if errs := schema.ValidateEntityProperties(context.Background(), nil, nil); errs != nil {
		t.Errorf("ValidateEntityProperties(nil,nil) = %v, want nil", errs)
	}
	if errs := schema.ValidateRelationProperties(context.Background(), nil, nil); errs != nil {
		t.Errorf("ValidateRelationProperties(nil,nil) = %v, want nil", errs)
	}
}
