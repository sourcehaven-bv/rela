package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Write-path validation runs on every entitymanager mutation, so its
// cost is part of every create/update (TKT-9Y4ZWS). The two benchmarks
// split the rule kinds: when/then rules are pure property checks; Lua
// rules pay for a fresh sandboxed state per rule evaluation — that
// per-write cost is exactly what these numbers make visible.

func benchTicketMeta(rules ...metamodel.ValidationRule) *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
		},
		Validations: rules,
	}
}

func benchTickets(n int) []*entity.Entity {
	out := make([]*entity.Entity, 0, n)
	for i := range n {
		e := entity.New(fmt.Sprintf("TKT-%04d", i), "ticket")
		e.SetString("status", []string{"ready", "draft"}[i%2])
		if i%3 != 0 { // leave a third without priority so rules fire
			e.SetString("priority", "high")
		}
		out = append(out, e)
	}
	return out
}

func BenchmarkCheck_WhenThen(b *testing.B) {
	meta := benchTicketMeta(metamodel.ValidationRule{
		Name:        "ready-needs-priority",
		Description: "Ready tickets must have priority",
		EntityType:  "ticket",
		When:        []string{"status=ready"},
		Then:        []string{"priority!="},
		Severity:    "error",
	})
	entities := benchTickets(100)
	svc := New(meta, newMockWorkspace().services(b.TempDir()))

	b.ReportAllocs()
	for b.Loop() {
		_ = svc.Check(context.Background(), entities, nil)
	}
}

func BenchmarkCheck_Lua(b *testing.B) {
	meta := benchTicketMeta(metamodel.ValidationRule{
		Name:        "ready-needs-priority-lua",
		Description: "Ready tickets must have priority (Lua)",
		EntityType:  "ticket",
		Lua: `
			if entity.properties.status == "ready" and
				(entity.properties.priority == nil or entity.properties.priority == "") then
				return { message = "priority required for ready tickets" }
			end
			return nil
		`,
		Severity: "error",
	})
	entities := benchTickets(100)
	svc := New(meta, newMockWorkspace().services(b.TempDir()))

	b.ReportAllocs()
	for b.Loop() {
		_ = svc.Check(context.Background(), entities, nil)
	}
}
