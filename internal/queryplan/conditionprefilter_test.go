package queryplan_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func conditionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"assignee": {Type: metamodel.PropertyTypeString},
				"status":   {Type: metamodel.PropertyTypeString},
				"count":    {Type: metamodel.PropertyTypeInteger},
				"watchers": {Type: metamodel.PropertyTypeString, List: true},
			}},
			"bug": {Properties: map[string]metamodel.PropertyDef{
				"assignee": {Type: metamodel.PropertyTypeString},
				"status":   {Type: metamodel.PropertyTypeString},
			}},
		},
	}
}

func TestConditionPrefilters(t *testing.T) {
	meta := conditionMeta()
	const identity = "PERS-JV"

	tests := []struct {
		name     string
		src      string
		types    []string
		identity string
		want     []store.PropPredicate
	}{
		{
			name:     "current-user equality resolves to the identity",
			src:      "entity.assignee == current_user.id",
			types:    []string{"task"},
			identity: identity,
			want: []store.PropPredicate{
				{Property: "assignee", Op: store.PropEqual, Value: identity, Scalar: true},
			},
		},
		{
			name:     "literal and identity push together",
			src:      "entity.status == 'open' and entity.assignee == current_user.id",
			types:    []string{"task"},
			identity: identity,
			want: []store.PropPredicate{
				{Property: "assignee", Op: store.PropEqual, Value: identity, Scalar: true},
				{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
			},
		},
		{
			name:     "pushes only across types that all declare the property as a string",
			src:      "entity.assignee == current_user.id",
			types:    []string{"task", "bug"},
			identity: identity,
			want: []store.PropPredicate{
				{Property: "assignee", Op: store.PropEqual, Value: identity, Scalar: true},
			},
		},
		{
			name:     "a property missing on one type pushes nothing",
			src:      "entity.count == 3 and entity.assignee == current_user.id",
			types:    []string{"task", "bug"},
			identity: identity,
			want: []store.PropPredicate{
				{Property: "assignee", Op: store.PropEqual, Value: identity, Scalar: true},
			},
		},

		// --- fail-closed and soundness ---
		{
			name:     "an empty identity pushes NOTHING, never an empty-string equality",
			src:      "entity.assignee == current_user.id",
			types:    []string{"task"},
			identity: "",
			want:     nil,
		},
		{
			name:     "an OR pushes nothing",
			src:      "entity.assignee == current_user.id or entity.status == 'open'",
			types:    []string{"task"},
			identity: identity,
			want:     nil,
		},
		{
			name:     "the is_current_user sugar is not a pushable equality shape",
			src:      "is_current_user(entity.assignee)",
			types:    []string{"task"},
			identity: identity,
			want:     nil,
		},
		{
			name:     "current_user.tool is never pushed",
			src:      "entity.status == current_user.tool",
			types:    []string{"task"},
			identity: identity,
			want:     nil,
		},
		{
			// A list property cannot even be COMPARED (the type checker
			// rejects it), so membership is spelled has_current_user — and a host
			// call is not an equality shape, so it pushes nothing.
			name:     "list membership is not pushed",
			src:      "has_current_user(entity.watchers)",
			types:    []string{"task"},
			identity: identity,
			want:     nil,
		},
		{
			name:     "no types pushes nothing",
			src:      "entity.assignee == current_user.id",
			types:    nil,
			identity: identity,
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := predicatefns.NewEvaluator(meta)
			// Compile against the first named type; the condition must
			// type-check on every type it will be evaluated against, which
			// is the rule CompileNextActions already enforces.
			compileType := "task"
			prog, err := ev.CompileWithCurrentUser(compileType, tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got := queryplan.ConditionPrefilters(prog, meta, tc.types, tc.identity)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d predicates %+v, want %d %+v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("predicate %d: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestConditionPrefilters_NilInputs covers the degenerate cases at the
// boundary rather than leaving them to panic in a handler.
func TestConditionPrefilters_NilInputs(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	prog, err := ev.CompileWithCurrentUser("task", "entity.assignee == current_user.id")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := queryplan.ConditionPrefilters(nil, meta, []string{"task"}, "x"); got != nil {
		t.Fatalf("nil program: got %+v, want nil", got)
	}
	if got := queryplan.ConditionPrefilters(prog, nil, []string{"task"}, "x"); got != nil {
		t.Fatalf("nil meta: got %+v, want nil", got)
	}
}
