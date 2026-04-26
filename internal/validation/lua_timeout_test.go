package validation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestLuaValidation_PerRuleTimeout covers AC4: each rule gets its
// own ~5s budget, so two back-to-back busy-loop rules total well
// under double the budget — proving the timeout doesn't accumulate
// across rules.
func TestLuaValidation_PerRuleTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running timeout test in -short mode")
	}
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "busy-1",
				EntityType: "ticket",
				Lua:        `while true do end`,
			},
			{
				Name:       "busy-2",
				EntityType: "ticket",
				Lua:        `while true do end`,
			},
		},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}
	svc := New(meta, ws.services(t.TempDir()))

	start := time.Now()
	result := svc.Check(context.Background(), entities, nil)
	elapsed := time.Since(start)

	if len(result.ScriptErrors) != 2 {
		t.Fatalf("got %d ScriptErrors, want 2", len(result.ScriptErrors))
	}
	// Each rule's ~5s timeout fires; total wall-clock is ~10s but
	// must finish well under 14s (sanity headroom).
	maxWall := 2*validationTimeout + 4*time.Second
	if elapsed > maxWall {
		t.Errorf("two timeouts ran for %v; want under %v (timeouts must not stack)",
			elapsed, maxWall)
	}
}

// TestLuaValidation_ParentContextCancellation covers AC8: canceling
// the parent ctx interrupts an in-flight Lua rule well within the
// 5s timeout.
func TestLuaValidation_ParentContextCancellation(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "busy",
				EntityType: "ticket",
				Lua:        `while true do end`,
			},
		},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}
	svc := New(meta, ws.services(t.TempDir()))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result := svc.Check(ctx, entities, nil)
	elapsed := time.Since(start)

	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result.ScriptErrors))
	}
	// Cancellation must arrive long before the 5s timeout would.
	if elapsed > 2*time.Second {
		t.Errorf("ran for %v; expected parent ctx cancellation to interrupt within ~150ms",
			elapsed)
	}
	se := result.ScriptErrors[0]
	if !strings.Contains(strings.ToLower(se.LuaMessage), "context") {
		t.Errorf("LuaMessage = %q, want it to mention context (canceled/cancelled)",
			se.LuaMessage)
	}
}
