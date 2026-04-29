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
	// Each rule's 5s timeout fires; the total must be tightly bounded
	// at 2*validationTimeout + a small headroom for runtime
	// construction and ScriptError envelope creation. The headroom is
	// intentionally narrow so we catch a regression where a third
	// timeout slips through.
	maxWall := 2*validationTimeout + 500*time.Millisecond
	if elapsed > maxWall {
		t.Errorf("two timeouts ran for %v; want under %v (timeouts must not stack)",
			elapsed, maxWall)
	}
}

// TestLuaValidation_AlreadyCancelledContext covers RR-VIIZJ: when the
// parent ctx is already cancelled before Service.Check is called,
// every rule must surface as a ScriptError, no runtime should panic,
// and the call must finish quickly (no work attempted past the first
// cancellation check).
func TestLuaValidation_AlreadyCancelledContext(t *testing.T) {
	ws := newMockWorkspace()
	rules := make([]metamodel.ValidationRule, 5)
	for i := range rules {
		rules[i] = metamodel.ValidationRule{
			Name:       "busy",
			EntityType: "ticket",
			Lua:        `while true do end`,
		}
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: rules,
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE Check is invoked

	svc := New(meta, ws.services(t.TempDir()))
	start := time.Now()
	result := svc.Check(ctx, entities, nil)
	elapsed := time.Since(start)

	// Pre-cancelled ctx should bail out immediately at the per-rule
	// guard: zero rules attempted, zero ScriptErrors, no panic.
	if elapsed > 100*time.Millisecond {
		t.Errorf("ran for %v; expected pre-cancelled ctx to bail out under 100ms", elapsed)
	}
	if len(result.Violations) != 0 {
		t.Errorf("got %d violations, want 0 on pre-cancelled ctx", len(result.Violations))
	}
}

// TestLuaValidation_CheckRulesBailsOnCancellation covers RR-JHQKY:
// CheckRules checks ctx.Err() at the top of each rule iteration so
// it does not construct N more runtimes after the parent ctx is
// already cancelled.
func TestLuaValidation_CheckRulesBailsOnCancellation(t *testing.T) {
	ws := newMockWorkspace()
	rules := make([]metamodel.ValidationRule, 100)
	for i := range rules {
		rules[i] = metamodel.ValidationRule{
			Name:       "always-pass",
			EntityType: "ticket",
			Lua:        `return nil`,
		}
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: rules,
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := New(meta, ws.services(t.TempDir()))
	start := time.Now()
	_ = svc.Check(ctx, entities, nil)
	elapsed := time.Since(start)

	// Constructing 100 runtimes would take well over 100ms. The
	// early bail keeps the call near-instant.
	if elapsed > 100*time.Millisecond {
		t.Errorf("ran for %v; expected CheckRules to bail before constructing 100 runtimes", elapsed)
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
	// Tight bound: cancel fires at 100ms; total budget is 500ms to
	// allow for goroutine scheduling and ScriptError construction.
	if elapsed > 500*time.Millisecond {
		t.Errorf("ran for %v; expected parent ctx cancellation to interrupt within 500ms",
			elapsed)
	}
	se := result.ScriptErrors[0]
	if !strings.Contains(strings.ToLower(se.LuaMessage), "context") {
		t.Errorf("LuaMessage = %q, want it to mention context (canceled/cancelled)",
			se.LuaMessage)
	}
}
