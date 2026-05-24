package analysis_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func orderableMeta(t *testing.T, mode metamodel.OrderableMode) *metamodel.Metamodel {
	t.Helper()
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"recipe": {Label: "Recipe"},
			"step":   {Label: "Step"},
		},
		Relations: map[string]metamodel.RelationDef{
			"has-step": {
				Label:     "has step",
				From:      []string{"recipe"},
				To:        []string{"step"},
				Orderable: mode,
			},
		},
	}
}

func addOrderedRelation(s store.Store, to string, order interface{}) {
	props := map[string]interface{}{}
	if order != nil {
		props[metamodel.OrderPropertyOut] = order
	}
	if _, err := s.CreateRelation(context.Background(), "REC-1", "has-step", to,
		&store.RelationData{Properties: props}); err != nil {
		panic(err)
	}
}

func TestCheckRelationOrder_DuplicateDetected(t *testing.T) {
	svc := newServiceWith(t, orderableMeta(t, metamodel.OrderableOutgoing), func(s store.Store) {
		addEntity(s, "REC-1", "recipe", nil)
		addEntity(s, "STP-1", "step", nil)
		addEntity(s, "STP-2", "step", nil)
		addOrderedRelation(s, "STP-1", 1.0)
		addOrderedRelation(s, "STP-2", 1.0) // duplicate
	})

	issues := svc.CheckRelationOrder(analysis.Options{})
	if len(issues) == 0 {
		t.Fatal("expected at least one duplicate issue")
	}
	foundDup := false
	for _, iss := range issues {
		if iss.Kind == "duplicate" {
			foundDup = true
		}
	}
	if !foundDup {
		t.Errorf("expected duplicate kind, got %+v", issues)
	}
}

func TestCheckRelationOrder_MissingDetected(t *testing.T) {
	svc := newServiceWith(t, orderableMeta(t, metamodel.OrderableOutgoing), func(s store.Store) {
		addEntity(s, "REC-1", "recipe", nil)
		addEntity(s, "STP-1", "step", nil)
		addEntity(s, "STP-2", "step", nil)
		addOrderedRelation(s, "STP-1", 1.0)
		addOrderedRelation(s, "STP-2", nil) // missing
	})

	issues := svc.CheckRelationOrder(analysis.Options{})
	foundMissing := false
	for _, iss := range issues {
		if iss.Kind == "missing" {
			foundMissing = true
			if !strings.Contains(iss.Property, "_order_out") {
				t.Errorf("missing issue not on _order_out: %+v", iss)
			}
		}
	}
	if !foundMissing {
		t.Errorf("expected missing kind, got %+v", issues)
	}
}

func TestCheckRelationOrder_NonOrderableSkipped(t *testing.T) {
	svc := newServiceWith(t, orderableMeta(t, metamodel.OrderableNone), func(s store.Store) {
		addEntity(s, "REC-1", "recipe", nil)
		addEntity(s, "STP-1", "step", nil)
		addOrderedRelation(s, "STP-1", nil)
	})

	issues := svc.CheckRelationOrder(analysis.Options{})
	if len(issues) != 0 {
		t.Errorf("non-orderable type should produce no issues, got %+v", issues)
	}
}
