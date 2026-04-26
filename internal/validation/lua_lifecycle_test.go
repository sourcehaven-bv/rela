package validation

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestLuaValidation_RuntimeHoistedAcrossEntities asserts that the
// per-rule runtime is built once per CheckRule invocation and reused
// across entities — a Lua-level top-level counter survives between
// iterations, which would not be possible if the runtime were
// rebuilt per (rule, entity).
func TestLuaValidation_RuntimeHoistedAcrossEntities(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "counts-invocations",
				EntityType: "ticket",
				// Uses a top-level (chunk-local) counter declared once
				// at first run. If the runtime is rebuilt per entity,
				// the script will see counter == 1 every time.
				Lua: `
					counter = (counter or 0) + 1
					if counter < 2 then
						return nil
					end
					return { message = "counter=" .. tostring(counter) }
				`,
			},
		},
	}

	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{}},
		{ID: "TKT-003", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	// Two entities should violate (counter reaches 2 on second entity
	// and stays > 1 thereafter). If runtime were rebuilt per entity,
	// every entity would see counter=1 and produce zero violations.
	if len(result.Violations) != 2 {
		t.Fatalf("got %d violations, want 2 (runtime should be hoisted across entities)",
			len(result.Violations))
	}
	if result.Violations[0].EntityID != entities[1].ID {
		t.Errorf("first violation entity = %s, want %s",
			result.Violations[0].EntityID, entities[1].ID)
	}
	if result.Violations[1].EntityID != entities[2].ID {
		t.Errorf("second violation entity = %s, want %s",
			result.Violations[1].EntityID, entities[2].ID)
	}
	if result.HasErrors() {
		t.Errorf("HasErrors = true, want false (no script errors expected)")
	}
}

// TestLuaValidation_FreshRuntimePerCheckCall asserts that two
// successive CheckRule invocations get independent runtimes — if a
// previous run left state behind in a process-global, that state
// must not leak into the next CheckRule call.
func TestLuaValidation_FreshRuntimePerCheckCall(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "counts-invocations",
				EntityType: "ticket",
				Lua: `
					counter = (counter or 0) + 1
					return { message = "counter=" .. tostring(counter) }
				`,
			},
		},
	}

	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, ws.services(t.TempDir()))

	// First run: counter starts at nil, becomes 1.
	first := svc.Check(context.Background(), entities, nil)
	if len(first.Violations) != 1 {
		t.Fatalf("first run: got %d violations, want 1", len(first.Violations))
	}
	wantFirst := "counter=1"
	if first.Violations[0].Description != wantFirst {
		t.Errorf("first run description = %q, want %q",
			first.Violations[0].Description, wantFirst)
	}

	// Second run: must also start fresh, not see the leaked counter
	// from the previous CheckRule (the runtime is built per CheckRule
	// call, then closed).
	second := svc.Check(context.Background(), entities, nil)
	if len(second.Violations) != 1 {
		t.Fatalf("second run: got %d violations, want 1", len(second.Violations))
	}
	if second.Violations[0].Description != wantFirst {
		t.Errorf("second run description = %q, want %q (runtime must be fresh per CheckRule)",
			second.Violations[0].Description, wantFirst)
	}
}
