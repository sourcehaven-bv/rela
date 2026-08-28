package computed_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/computed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func meta(props map[string]metamodel.PropertyDef) *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"item": {Properties: props},
	}}
}

func TestCompileEvaluate_ChainedAndPortable(t *testing.T) {
	m := meta(map[string]metamodel.PropertyDef{
		"a":     {Type: metamodel.PropertyTypeInteger},
		"b":     {Type: metamodel.PropertyTypeInteger, Computed: "entity.a * 2"},
		"c":     {Type: metamodel.PropertyTypeInteger, Computed: "entity.b + 1"},
		"name":  {Type: metamodel.PropertyTypeString},
		"label": {Type: metamodel.PropertyTypeString, Computed: "entity.name .. '!'"},
	})
	set, err := computed.Compile(m)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	e := &entity.Entity{ID: "I-1", Type: "item", Properties: map[string]any{"a": 4, "name": "Ada"}}
	if err := set.Evaluate(context.Background(), e); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if e.Properties["b"] != int64(8) || e.Properties["c"] != int64(9) || e.Properties["label"] != "Ada!" {
		t.Fatalf("properties = %#v", e.Properties)
	}
	if !set.SQLPortable("item", "c") {
		t.Fatal("c should be SQL-portable")
	}
}

func TestCompile_RruleIsValidButNotPortable(t *testing.T) {
	m := meta(map[string]metamodel.PropertyDef{
		"rule":  {Type: metamodel.PropertyTypeRrule},
		"start": {Type: metamodel.PropertyTypeDate},
		"next":  {Type: metamodel.PropertyTypeDate, Computed: "rrule_next(entity.rule, entity.start)"},
	})
	set, err := computed.CompileWithClock(m, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if set.SQLPortable("item", "next") {
		t.Fatal("rrule_next must be host-only")
	}
	e := &entity.Entity{ID: "I-1", Type: "item", Properties: map[string]any{
		"rule": "FREQ=DAILY;COUNT=3", "start": "2026-01-01",
	}}
	if err := set.Evaluate(context.Background(), e); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := e.Properties["next"]; got != "2026-01-02" {
		t.Fatalf("next = %v", got)
	}
}

func TestCompile_RejectsCyclesAndTypeMismatch(t *testing.T) {
	_, err := computed.Compile(meta(map[string]metamodel.PropertyDef{
		"a": {Type: metamodel.PropertyTypeInteger, Computed: "entity.b + 1"},
		"b": {Type: metamodel.PropertyTypeInteger, Computed: "entity.a + 1"},
	}))
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}
	_, err = computed.Compile(meta(map[string]metamodel.PropertyDef{
		"a": {Type: metamodel.PropertyTypeInteger, Computed: "'wrong'"},
	}))
	if err == nil || !strings.Contains(err.Error(), "must be int") {
		t.Fatalf("type error = %v", err)
	}
}
