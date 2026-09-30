package schema

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Compile-time check that StoreCounter satisfies TypeCounter.
var _ TypeCounter = (*StoreCounter)(nil)

// mustCounter builds a StoreCounter or fails the test.
func mustCounter(t *testing.T, st TypeCounts, families store.WorldScope) *StoreCounter {
	t.Helper()
	c, err := NewStoreCounter(context.Background(), st, families)
	if err != nil {
		t.Fatalf("NewStoreCounter: %v", err)
	}
	return c
}

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
	s.CreateRelation(ctx, entity.RelationKey{From: "DEC-001", Type: "implements", To: "REQ-001"}, nil)
	s.CreateRelation(ctx, entity.RelationKey{From: "REQ-002", Type: "depends-on", To: "REQ-001"}, nil)

	// Run Analyze with StoreCounter — same metamodel as newTestMetamodel()
	meta := newTestMetamodel()
	counter := mustCounter(t, s, store.TrivialScope())
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
	s.CreateRelation(ctx, entity.RelationKey{From: "DEC-001", Type: "implements", To: "REQ-001"}, nil)
	s.CreateRelation(ctx, entity.RelationKey{From: "REQ-002", Type: "depends-on", To: "REQ-001"}, nil)

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

	counter := mustCounter(t, s, store.TrivialScope())
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

// failingCounts fails every count.
type failingCounts struct{}

func (failingCounts) CountEntities(context.Context, store.EntityQuery) (int, error) {
	return 0, errors.New("boom")
}

func (failingCounts) CountRelations(context.Context, store.RelationQuery) (int, error) {
	return 0, errors.New("boom")
}

// TestStoreCounter_RejectsUnsetFamilies pins that an unset scope is a
// construction error, not a report listing every type as unused.
func TestStoreCounter_RejectsUnsetFamilies(t *testing.T) {
	if _, err := NewStoreCounter(context.Background(), memstore.New(), store.WorldScope{}); err == nil {
		t.Fatal("NewStoreCounter accepted an unset families scope")
	}
	if _, err := NewStoreCounter(context.Background(), nil, store.TrivialScope()); err == nil {
		t.Fatal("NewStoreCounter accepted a nil store")
	}
}

// TestStoreCounter_ErrSurfacesFailedCount pins that a failed count is kept
// for Err rather than reading as a silent zero.
func TestStoreCounter_ErrSurfacesFailedCount(t *testing.T) {
	c := mustCounter(t, failingCounts{}, store.TrivialScope())
	if c.Err() != nil {
		t.Fatalf("Err before counting = %v, want nil", c.Err())
	}
	c.CountByEntityType("requirement")
	c.CountByRelationType("implements")
	if err := c.Err(); err == nil || !strings.Contains(err.Error(), "entity type requirement") {
		t.Fatalf("Err = %v, want the first failed count", err)
	}
}
