package validation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// condMeta declares a date-bearing type plus a rule slot the tests fill
// in per case.
func condMeta(rules ...metamodel.ValidationRule) *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Properties: map[string]metamodel.PropertyDef{
					"due":    {Type: metamodel.PropertyTypeDate},
					"status": {Type: metamodel.PropertyTypeString},
					"owner":  {Type: metamodel.PropertyTypeString},
				},
			},
		},
		Validations: rules,
	}
}

func taak(id string, props map[string]any) *entity.Entity {
	return &entity.Entity{ID: id, Type: "taak", Properties: props}
}

// TestThenCondition pins that `then_condition:` asserts an expression
// over the entities a rule selects — the half a filter clause cannot
// express once date arithmetic is involved.
func TestThenCondition(t *testing.T) {
	meta := condMeta(metamodel.ValidationRule{
		Name:        "open-tasks-need-an-owner",
		Description: "an open taak must name an owner",
		EntityType:  "taak",
		When:        []string{"status=open"},
		// Expression rather than `then: ["owner!="]` so the assertion can
		// use composition the filter dialect has no syntax for.
		ThenCondition: "entity.owner ~= nil and entity.owner ~= ''",
		Severity:      "error",
	})
	svc := New(meta, lua.ReadDeps{})

	entities := []*entity.Entity{
		taak("T-1", map[string]any{"status": "open", "owner": "alice"}),
		taak("T-2", map[string]any{"status": "open"}), // violates
		taak("T-3", map[string]any{"status": "done"}), // not selected
	}

	violations := svc.Check(context.Background(), entities, nil).Violations
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(violations), violations)
	}
	if violations[0].EntityID != "T-2" {
		t.Errorf("violation on %s, want T-2", violations[0].EntityID)
	}
}

// TestWhenCondition pins that `when_condition:` narrows which entities a
// rule applies to, ANDed with any `when:` clauses.
func TestWhenCondition(t *testing.T) {
	meta := condMeta(metamodel.ValidationRule{
		Name:        "recent-tasks-need-an-owner",
		Description: "a task due within a week must name an owner",
		EntityType:  "taak",
		When:        []string{"status=open"},
		// Only tasks due on or before 2026-08-25 are in scope.
		WhenCondition: "entity.due <= '2026-08-25'",
		Then:          []string{"owner!="},
		Severity:      "error",
	})
	svc := New(meta, lua.ReadDeps{})

	entities := []*entity.Entity{
		// selected by both when: and when_condition:, and violates then:
		taak("T-1", map[string]any{"status": "open", "due": "2026-08-20"}),
		// due later — when_condition: excludes it
		taak("T-2", map[string]any{"status": "open", "due": "2026-12-01"}),
		// when: excludes it
		taak("T-3", map[string]any{"status": "done", "due": "2026-08-20"}),
	}

	violations := svc.Check(context.Background(), entities, nil).Violations
	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(violations), violations)
	}
	if violations[0].EntityID != "T-1" {
		t.Errorf("violation on %s, want T-1", violations[0].EntityID)
	}
}

// TestConditionAbsent_Unchanged pins that a rule with neither condition
// key behaves exactly as before.
func TestConditionAbsent_Unchanged(t *testing.T) {
	meta := condMeta(metamodel.ValidationRule{
		Name:        "open-tasks-need-an-owner",
		Description: "an open taak must name an owner",
		EntityType:  "taak",
		When:        []string{"status=open"},
		Then:        []string{"owner!="},
		Severity:    "error",
	})
	svc := New(meta, lua.ReadDeps{})

	entities := []*entity.Entity{
		taak("T-1", map[string]any{"status": "open", "owner": "alice"}),
		taak("T-2", map[string]any{"status": "open"}),
	}
	violations := svc.Check(context.Background(), entities, nil).Violations
	if len(violations) != 1 || violations[0].EntityID != "T-2" {
		t.Errorf("plain when/then rule changed behavior: %+v", violations)
	}
}
