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
			name:     "the is_current_user sugar lowers to the same scalar equality",
			src:      "is_current_user(entity.assignee)",
			types:    []string{"task"},
			identity: identity,
			want: []store.PropPredicate{
				{Property: "assignee", Op: store.PropEqual, Value: identity, Scalar: true},
			},
		},
		{
			// A NON-scalar PropEqual: every backend reads that as "some
			// element equals" for a list value, which is exactly the Go
			// semantics of has_current_user.
			name:     "has_current_user lowers to a non-scalar equality (list membership)",
			src:      "has_current_user(entity.watchers)",
			types:    []string{"task"},
			identity: identity,
			want: []store.PropPredicate{
				{Property: "watchers", Op: store.PropEqual, Value: identity, Scalar: false},
			},
		},
		{
			name:     "membership is gated on the property being a string list on every type",
			src:      "has_current_user(entity.watchers)",
			types:    []string{"task", "bug"},
			identity: identity,
			want:     nil,
		},
		{
			name:     "sugar without an identity pushes nothing",
			src:      "is_current_user(entity.assignee) and has_current_user(entity.watchers)",
			types:    []string{"task"},
			identity: "",
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

// TestConditionIndexProperties pins the index-inference half of the
// eligibility decision: scalar equalities derive an index column,
// memberships and everything ConditionPrefilters would refuse do not.
func TestConditionIndexProperties(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	tests := []struct {
		name  string
		src   string
		types []string
		want  []string
	}{
		{"identity equality", "entity.assignee == current_user.id", []string{"task"}, []string{"assignee"}},
		{"sugar equality", "is_current_user(entity.assignee)", []string{"task"}, []string{"assignee"}},
		{"literal equality", "entity.status == 'open'", []string{"task"}, []string{"status"}},
		{"membership is not indexable", "has_current_user(entity.watchers)", []string{"task"}, nil},
		{"mixed keeps the scalar ones",
			"entity.status == 'open' and is_current_user(entity.assignee) and has_current_user(entity.watchers)",
			[]string{"task"}, []string{"assignee", "status"}},
		{"OR derives nothing", "entity.status == 'open' or is_current_user(entity.assignee)", []string{"task"}, nil},
		{"typed equality derives nothing", "entity.count == 3", []string{"task"}, nil},
		{"multi-type keeps only the shared string property",
			"entity.count == 3 and entity.assignee == current_user.id", []string{"task", "bug"}, []string{"assignee"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got := queryplan.ConditionIndexProperties(prog, meta, tc.types)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestConditionPrefilters_AgreesWithIndexProperties is the drift guard
// the CLAUDE.md rule asks for: every scalar predicate the runtime pushes
// is a column the static index derives, and vice versa.
func TestConditionPrefilters_AgreesWithIndexProperties(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	srcs := []string{
		"entity.assignee == current_user.id",
		"is_current_user(entity.assignee)",
		"has_current_user(entity.watchers)",
		"entity.status == 'open' and is_current_user(entity.assignee) and has_current_user(entity.watchers)",
		"entity.status == 'open' or is_current_user(entity.assignee)",
		"entity.count == 3 and entity.assignee == current_user.id",
	}
	for _, src := range srcs {
		t.Run(src, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			types := []string{"task"}
			var pushedScalar []string
			for _, p := range queryplan.ConditionPrefilters(prog, meta, types, "PERS-JV") {
				if p.Scalar {
					pushedScalar = append(pushedScalar, p.Property)
				}
			}
			indexed := queryplan.ConditionIndexProperties(prog, meta, types)
			if len(pushedScalar) != len(indexed) {
				t.Fatalf("pushed %v, indexed %v", pushedScalar, indexed)
			}
			for i := range indexed {
				if pushedScalar[i] != indexed[i] {
					t.Fatalf("pushed %v, indexed %v", pushedScalar, indexed)
				}
			}
		})
	}
}
