package predicatefns_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

func evalMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"item": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: metamodel.PropertyTypeString},
				"count":  {Type: metamodel.PropertyTypeInteger},
				"active": {Type: metamodel.PropertyTypeBoolean},
				"due":    {Type: metamodel.PropertyTypeDate},
				"tags":   {Type: metamodel.PropertyTypeString, List: true},
			}},
		},
	}
}

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC) }
}

// TestEvaluator_Compile_And_Matches exercises the raw-expression path,
// the cache (a second Compile of the same source returns the cached
// Program), and per-entity evaluation.
func TestEvaluator_Compile_And_Matches(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(evalMeta(), fixedClock())

	prog1, err := ev.Compile("item", "entity.count > 5")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	prog2, err := ev.Compile("item", "entity.count > 5")
	if err != nil {
		t.Fatalf("compile 2: %v", err)
	}
	if prog1 != prog2 {
		t.Error("expected the second Compile of an identical source to return the cached Program")
	}

	cases := []struct {
		count any
		want  bool
	}{
		{10, true}, {int64(3), false}, {"7", true},
	}
	for _, tc := range cases {
		got, err := ev.Matches(context.Background(), prog1, "item", "I-1", map[string]any{"count": tc.count})
		if err != nil {
			t.Fatalf("matches count=%v: %v", tc.count, err)
		}
		if got != tc.want {
			t.Errorf("count=%v: got %v want %v", tc.count, got, tc.want)
		}
	}

	// A value that binds Nil (non-numeric string on an int field) makes
	// the ordered comparison an eval error — surfaced, not swallowed.
	if _, err := ev.Matches(context.Background(), prog1, "item", "I-2", map[string]any{"count": "x"}); err == nil {
		t.Error("expected eval error for a nil-bound int compared with >")
	}
}

// TestEvaluator_CompileFilter routes legacy filter clauses through the
// transpiler + compile, and AndFilters combines them.
func TestEvaluator_CompileFilter(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(evalMeta(), fixedClock())
	fs, err := filter.ParseAll([]string{"title=hello", "count>3"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := ev.CompileFilter("item", fs)
	if err != nil {
		t.Fatalf("compilefilter: %v", err)
	}
	got, err := ev.Matches(context.Background(), prog, "item", "I-1",
		map[string]any{"title": "hello", "count": 5})
	if err != nil {
		t.Fatalf("matches: %v", err)
	}
	if !got {
		t.Error("expected title=hello AND count>3 to match")
	}
	got, _ = ev.Matches(context.Background(), prog, "item", "I-2",
		map[string]any{"title": "hello", "count": 2})
	if got {
		t.Error("expected count=2 to fail count>3")
	}
}

// TestEvaluator_UnknownType errors on an unknown entity type.
func TestEvaluator_UnknownType(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(evalMeta(), fixedClock())
	if _, err := ev.Compile("nope", "entity.count > 1"); err == nil {
		t.Error("expected error for unknown entity type")
	}
}

// TestEvaluator_Today proves today() reads the clock per Matches.
func TestEvaluator_Today(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(evalMeta(), fixedClock())
	prog, err := ev.Compile("item", "entity.due < today()")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got, err := ev.Matches(context.Background(), prog, "item", "I-1",
		map[string]any{"due": "2026-01-01"})
	if err != nil {
		t.Fatalf("matches: %v", err)
	}
	if !got {
		t.Error("2026-01-01 should be < today() (2026-07-18)")
	}
}

// TestEntityRecord_Coercions exercises the shared binder directly for
// every declared type + list + id/type pseudo-fields.
func TestEntityRecord_Coercions(t *testing.T) {
	meta := evalMeta()
	def, _ := meta.GetEntityDef("item")
	due, _ := time.Parse("2006-01-02", "2026-03-04")

	rec := predicatefns.EntityRecord(meta, def, "I-9", "item", map[string]any{
		"title":  "hi",
		"count":  int64(42),
		"active": true,
		"due":    due, // time.Time shape
		"tags":   []any{"a", "b"},
	})
	r, ok := rec.(predicate.Record)
	if !ok {
		t.Fatalf("EntityRecord returned %T, want Record", rec)
	}

	// kindOf names the concrete Value variant for assertions.
	kindOf := func(v predicate.Value) string {
		switch v.(type) {
		case predicate.String:
			return "string"
		case predicate.Int:
			return "int"
		case predicate.Bool:
			return "bool"
		case predicate.Date:
			return "date"
		case predicate.List:
			return "list"
		case predicate.Nil:
			return "nil"
		default:
			return "?"
		}
	}
	check := func(field, wantKind string) {
		v, present := r.Get(field)
		if !present {
			t.Errorf("field %q missing", field)
			return
		}
		if got := kindOf(v); got != wantKind {
			t.Errorf("field %q kind = %s, want %s", field, got, wantKind)
		}
	}
	check("id", "string")
	check("type", "string")
	check("title", "string")
	check("count", "int")
	check("active", "bool")
	check("due", "date")
	check("tags", "list")

	// String-shaped date + string-shaped int + []string list.
	rec2 := predicatefns.EntityRecord(meta, def, "I-10", "item", map[string]any{
		"due":   "2026-03-04",
		"count": "7",
		"tags":  []string{"x"},
	})
	r2 := rec2.(predicate.Record)
	if v, _ := r2.Get("due"); kindOf(v) != "date" {
		t.Errorf("string due should coerce to Date, got %s", kindOf(v))
	}
	if v, _ := r2.Get("count"); kindOf(v) != "int" {
		t.Errorf("string count should coerce to Int, got %s", kindOf(v))
	}

	// Off-type values bind Nil (fail-soft).
	rec3 := predicatefns.EntityRecord(meta, def, "I-11", "item", map[string]any{
		"count":  1.5,              // fractional -> Nil
		"due":    12345,            // non-date -> Nil
		"active": map[string]int{}, // non-bool -> Nil
	})
	r3 := rec3.(predicate.Record)
	for _, f := range []string{"count", "due", "active"} {
		if v, _ := r3.Get(f); kindOf(v) != "nil" {
			t.Errorf("off-type %q should bind Nil, got %s", f, kindOf(v))
		}
	}
}
