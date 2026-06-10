package validation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:      "Ticket",
				IDPrefixes: []string{"TKT-"},
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "ready-needs-priority",
				Description: "Ready tickets must have priority",
				EntityType:  "ticket",
				When:        []string{"status=ready"},
				Then:        []string{"priority!="},
				Severity:    "error",
			},
		},
	}

	entities := []*entity.Entity{
		{
			ID:   "TKT-001",
			Type: "ticket",
			Properties: map[string]interface{}{
				"title":    "Valid ticket",
				"status":   "ready",
				"priority": "high",
			},
		},
		{
			ID:   "TKT-002",
			Type: "ticket",
			Properties: map[string]interface{}{
				"title":  "Invalid ticket",
				"status": "ready",
				// missing priority
			},
		},
		{
			ID:   "TKT-003",
			Type: "ticket",
			Properties: map[string]interface{}{
				"title":  "Draft ticket",
				"status": "draft",
				// no priority needed for draft
			},
		},
	}

	svc := New(meta, lua.ReadDeps{})

	t.Run("finds violations", func(t *testing.T) {
		violations := svc.Check(context.Background(), entities, nil).Violations
		if len(violations) != 1 {
			t.Errorf("got %d violations, want 1", len(violations))
		}
		if len(violations) > 0 && violations[0].EntityID != "TKT-002" {
			t.Errorf("violation entity = %s, want TKT-002", violations[0].EntityID)
		}
	})

	t.Run("scope filters violations", func(t *testing.T) {
		// Only check TKT-001 (valid) and TKT-003 (not matching when)
		scope := map[string]bool{"TKT-001": true, "TKT-003": true}
		violations := svc.Check(context.Background(), entities, scope).Violations
		if len(violations) != 0 {
			t.Errorf("got %d violations, want 0", len(violations))
		}
	})

	t.Run("scope includes violation", func(t *testing.T) {
		scope := map[string]bool{"TKT-002": true}
		violations := svc.Check(context.Background(), entities, scope).Violations
		if len(violations) != 1 {
			t.Errorf("got %d violations, want 1", len(violations))
		}
	})
}

func TestCheckWarnings(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {
				Label:      "Document",
				IDPrefixes: []string{"DOC-"},
				Properties: map[string]metamodel.PropertyDef{
					"reviewed": {Type: "boolean"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "should-be-reviewed",
				Description: "Documents should be reviewed",
				EntityType:  "doc",
				Then:        []string{"reviewed=true"},
				Severity:    "warning",
			},
		},
	}

	entities := []*entity.Entity{
		{
			ID:         "DOC-001",
			Type:       "doc",
			Properties: map[string]interface{}{"reviewed": false},
		},
	}

	svc := New(meta, lua.ReadDeps{})
	violations := svc.Check(context.Background(), entities, nil).Violations

	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("severity = %s, want warning", violations[0].Severity)
	}
}

func TestCountBySeverity(t *testing.T) {
	t.Parallel()
	violations := []Violation{
		{Severity: "error"},
		{Severity: "error"},
		{Severity: "warning"},
	}

	errors, warnings := CountBySeverity(violations)

	if errors != 2 {
		t.Errorf("errors = %d, want 2", errors)
	}
	if warnings != 1 {
		t.Errorf("warnings = %d, want 1", warnings)
	}
}

func TestRules(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Validations: []metamodel.ValidationRule{
			{Name: "rule-1"},
			{Name: "rule-2"},
		},
	}

	svc := New(meta, lua.ReadDeps{})
	rules := svc.Rules()

	if len(rules) != 2 {
		t.Errorf("got %d rules, want 2", len(rules))
	}
}

func TestNoRules(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{}

	svc := New(meta, lua.ReadDeps{})
	violations := svc.Check(context.Background(), []*entity.Entity{{ID: "X", Type: "x"}}, nil).Violations

	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0", len(violations))
	}
}

func TestAllEntityTypes(t *testing.T) {
	t.Parallel()
	// Rule without entity_type applies to all entities
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc":    {Label: "Document", Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}}},
			"ticket": {Label: "Ticket", Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "must-have-status",
				Description: "All entities must have status",
				// No EntityType - applies to all
				Then:     []string{"status!="},
				Severity: "error",
			},
		},
	}

	entities := []*entity.Entity{
		{ID: "DOC-001", Type: "doc", Properties: map[string]interface{}{"status": "draft"}},
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}}, // missing status
	}

	svc := New(meta, lua.ReadDeps{})
	violations := svc.Check(context.Background(), entities, nil).Violations

	if len(violations) != 1 {
		t.Errorf("got %d violations, want 1", len(violations))
	}
	if len(violations) > 0 && violations[0].EntityID != "TKT-001" {
		t.Errorf("violation entity = %s, want TKT-001", violations[0].EntityID)
	}
}
