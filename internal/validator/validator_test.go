package validator_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

func newTestValidator(t *testing.T) *validator.GenericValidator {
	t.Helper()
	s := memstore.New()
	ctx := context.Background()

	_ = s.CreateEntity(ctx, &entity.Entity{
		ID: "REQ-1", Type: "requirement",
		Properties: map[string]interface{}{"status": "approved", "owner": "alice"},
	})
	_ = s.CreateEntity(ctx, &entity.Entity{
		ID: "REQ-2", Type: "requirement",
		Properties: map[string]interface{}{"status": "approved"}, // missing owner
	})
	_ = s.CreateEntity(ctx, &entity.Entity{
		ID: "REQ-3", Type: "requirement",
		Properties: map[string]interface{}{"status": "draft"}, // status filter won't match
	})
	_ = s.CreateEntity(ctx, &entity.Entity{
		ID: "DEC-1", Type: "decision",
		Properties: map[string]interface{}{"status": "approved"},
	})

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label: "Requirement",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
					"owner":  {Type: "string"},
				},
			},
			"decision": {
				Label: "Decision",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
					"owner":  {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "approved-requires-owner",
				Description: "Approved requirements must have an owner",
				EntityType:  "requirement",
				When:        []string{"status=approved"},
				Then:        []string{"owner!="},
				Severity:    "error",
			},
		},
	}

	// Minimal Lua services; none of our validation rules use Lua so the
	// individual service fields are not exercised here.
	deps := lua.ReadDeps{Meta: meta}

	return validator.New(s, meta, deps)
}

func TestGenericValidator_CheckRule(t *testing.T) {
	v := newTestValidator(t)
	rule := metamodel.ValidationRule{
		Name:       "approved-requires-owner",
		EntityType: "requirement",
		When:       []string{"status=approved"},
		Then:       []string{"owner!="},
	}

	ids, err := v.CheckRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("CheckRule error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "REQ-2" {
		t.Errorf("expected [REQ-2], got %v", ids)
	}
}

func TestGenericValidator_CheckAll(t *testing.T) {
	v := newTestValidator(t)
	vios, err := v.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("CheckAll error: %v", err)
	}

	if len(vios) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(vios), vios)
	}
	if vios[0].EntityID != "REQ-2" {
		t.Errorf("expected REQ-2, got %s", vios[0].EntityID)
	}
	if vios[0].Severity != "error" {
		t.Errorf("severity = %q, want error", vios[0].Severity)
	}
	if vios[0].RuleName != "approved-requires-owner" {
		t.Errorf("rule name = %q", vios[0].RuleName)
	}
}

func TestGenericValidator_CheckRule_NoMatches(t *testing.T) {
	v := newTestValidator(t)
	rule := metamodel.ValidationRule{
		Name:       "no-match",
		EntityType: "requirement",
		When:       []string{"status=draft"},
		Then:       []string{"owner!="},
	}
	ids, err := v.CheckRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("CheckRule error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "REQ-3" {
		t.Errorf("expected [REQ-3] (draft has no owner), got %v", ids)
	}
}

func TestGenericValidator_CheckRule_WrongEntityType(t *testing.T) {
	v := newTestValidator(t)
	// Rule scoped to decision type; only DEC-1 is candidate, has owner=""
	rule := metamodel.ValidationRule{
		Name:       "decisions-need-owner",
		EntityType: "decision",
		Then:       []string{"owner!="},
	}
	ids, err := v.CheckRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("CheckRule error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "DEC-1" {
		t.Errorf("expected [DEC-1], got %v", ids)
	}
}
