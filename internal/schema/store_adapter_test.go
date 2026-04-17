package schema

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Compile-time check that StoreCounter satisfies TypeCounter.
var _ TypeCounter = (*StoreCounter)(nil)

func TestStoreCounter_Analyze(t *testing.T) {
	s := memstore.New()
	ctx := context.Background()

	// Seed entities
	e1 := entity.New("REQ-001", "requirement")
	e1.SetString("title", "First")
	e1.SetString("status", "draft")
	s.CreateEntity(ctx, e1)

	e2 := entity.New("REQ-002", "requirement")
	e2.SetString("title", "Second")
	s.CreateEntity(ctx, e2)

	e3 := entity.New("DEC-001", "decision")
	e3.SetString("title", "Decision")
	s.CreateEntity(ctx, e3)

	// Seed relations
	s.CreateRelation(ctx, "DEC-001", "implements", "REQ-001", nil)
	s.CreateRelation(ctx, "REQ-002", "depends-on", "REQ-001", nil)

	// Run Analyze with StoreCounter — same metamodel as newTestMetamodel()
	meta := newTestMetamodel()
	counter := &StoreCounter{Store: s}
	result := Analyze(meta, counter, nil, 0)

	// unused-type has no instances
	if len(result.UnusedEntityTypes) != 1 {
		t.Fatalf("expected 1 unused entity type, got %d", len(result.UnusedEntityTypes))
	}
	if result.UnusedEntityTypes[0].Name != "unused-type" {
		t.Errorf("expected unused-type, got %s", result.UnusedEntityTypes[0].Name)
	}

	// unused-relation has no instances
	if len(result.UnusedRelationTypes) != 1 {
		t.Fatalf("expected 1 unused relation type, got %d", len(result.UnusedRelationTypes))
	}
	if result.UnusedRelationTypes[0].Name != "unused-relation" {
		t.Errorf("expected unused-relation, got %s", result.UnusedRelationTypes[0].Name)
	}

	// unused-enum is not referenced by any property
	if len(result.UnusedCustomTypes) != 1 {
		t.Fatalf("expected 1 unused custom type, got %d: %v", len(result.UnusedCustomTypes), result.UnusedCustomTypes)
	}
}

func TestStoreCounter_LowUsage(t *testing.T) {
	s := memstore.New()
	ctx := context.Background()

	s.CreateEntity(ctx, entity.New("REQ-001", "requirement"))
	s.CreateEntity(ctx, entity.New("REQ-002", "requirement"))
	s.CreateEntity(ctx, entity.New("DEC-001", "decision"))
	s.CreateRelation(ctx, "DEC-001", "implements", "REQ-001", nil)
	s.CreateRelation(ctx, "REQ-002", "depends-on", "REQ-001", nil)

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {},
			"decision":    {},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {From: []string{"decision"}, To: []string{"requirement"}},
			"depends-on": {From: []string{"requirement"}, To: []string{"requirement"}},
		},
		Types: map[string]metamodel.CustomType{},
	}

	counter := &StoreCounter{Store: s}
	result := Analyze(meta, counter, nil, 1)

	// decision has 1 instance → low usage at threshold=1
	var found bool
	for _, u := range result.LowUsageEntityTypes {
		if u.Name == "decision" {
			found = true
			if u.Count != 1 {
				t.Errorf("expected count 1, got %d", u.Count)
			}
		}
	}
	if !found {
		t.Error("expected decision in low usage types")
	}
}
