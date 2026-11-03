package predicatefns_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

func collectionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: metamodel.PropertyTypeString},
			}},
			"project": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: metamodel.PropertyTypeString},
				"budget": {Type: metamodel.PropertyTypeInteger},
				"rank":   {Type: metamodel.PropertyTypeInteger},
			}},
			"note": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: metamodel.PropertyTypeString},
				"rank":  {Type: metamodel.PropertyTypeString},
			}},
		},
	}
}

// TestEvaluator_CompileForCollection covers the collection Env: entity.type
// and entity.id are readable, a property only another collected type
// declares binds nil, and a property no collected type declares is still a
// compile error.
func TestEvaluator_CompileForCollection(t *testing.T) {
	types := []string{"task", "project", "note"}
	cases := []struct {
		name, typ, src string
		props          map[string]any
		want           bool
	}{
		{"type and own property", "task", "entity.type == 'task' and entity.status == 'done'",
			map[string]any{"status": "done"}, true},
		{"type selects", "project", "entity.type == 'task' and entity.status == 'done'",
			map[string]any{"status": "done"}, false},
		{"id is readable", "task", "entity.id == 'T-1'", nil, true},
		{"foreign property is nil: equality false", "note", "entity.status == 'done'", nil, false},
		{"foreign property is nil: not-equal true", "note", "entity.status ~= 'done'", nil, true},
		{"foreign property is nil: nil test", "task", "entity.budget == nil", nil, true},
		{"disjunction over types", "project",
			"entity.type == 'project' or (entity.type == 'task' and entity.status ~= 'done')",
			map[string]any{"status": "done"}, true},
	}
	ev := predicatefns.NewEvaluator(collectionMeta())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := ev.CompileForCollection(tc.typ, types, tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := ev.Matches(context.Background(), prog, tc.typ, "T-1", tc.props)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("a property no collected type declares does not compile", func(t *testing.T) {
		if _, err := ev.CompileForCollection("task", types, "entity.stauts == 'done'"); err == nil {
			t.Fatal("want a compile error")
		}
	})
	t.Run("own declaration wins over another type's", func(t *testing.T) {
		// rank is an integer on project and a string on note.
		if _, err := ev.CompileForCollection("project", types, "entity.rank > 5"); err != nil {
			t.Fatalf("project: %v", err)
		}
		if _, err := ev.CompileForCollection("note", types, "entity.rank > 5"); err == nil {
			t.Fatal("note: a number comparison on a string property compiled")
		}
	})
	t.Run("an ordered comparison on a foreign property does not compile", func(t *testing.T) {
		// task lacks budget; at runtime nil > 5 could only fail.
		if _, err := ev.CompileForCollection("task", types, "entity.budget > 5"); err == nil {
			t.Fatal("an ordered comparison on a nil-only field compiled")
		}
	})
	t.Run("the plain Compile Env is unchanged", func(t *testing.T) {
		if _, err := ev.Compile("task", "entity.type == 'task'"); err == nil {
			t.Fatal("entity.type compiled outside the collection Env")
		}
	})
}
