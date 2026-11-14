package queryplan_test

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestLowerCondition(t *testing.T) {
	meta := conditionMeta()
	const identity = "PERS-JV"

	type trav struct {
		path    string
		negated bool
	}
	tests := []struct {
		name      string
		src       string
		identity  string
		wantOK    bool
		wantProps []store.PropPredicate
		wantTrav  []trav
	}{
		{
			name:   "what LowerScope lowers lowers the same",
			src:    "entity.status == 'open' and related(entity, 'implements') and is_current_user(entity.assignee)",
			wantOK: true, identity: identity,
			wantProps: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
				{Property: "assignee", Op: store.PropEqual, Value: identity, Scalar: true},
			},
			wantTrav: []trav{{"implements", false}},
		},
		{
			name: "an inequality lowers to not-equal-or-empty", src: "entity.status ~= 'done' and entity.status ~= 'wont-fix'",
			wantOK: true,
			wantProps: []store.PropPredicate{
				{Property: "status", Op: store.PropNotEqualOrEmpty, Value: "done", Scalar: true},
				{Property: "status", Op: store.PropNotEqualOrEmpty, Value: "wont-fix", Scalar: true},
			},
		},
		{
			name: "an inequality against current_user binds the identity", src: "entity.assignee ~= current_user.id",
			identity: identity, wantOK: true,
			wantProps: []store.PropPredicate{{Property: "assignee", Op: store.PropNotEqualOrEmpty, Value: identity, Scalar: true}},
		},
		{
			name: "a membership lowers to a list equality", src: "has_current_user(entity.watchers)",
			identity: identity, wantOK: true,
			wantProps: []store.PropPredicate{{Property: "watchers", Op: store.PropEqual, Value: identity}},
		},
		{
			name: "not related lowers negated", src: "not related(entity, 'implements')",
			wantOK: true, wantTrav: []trav{{"implements", true}},
		},
		{name: "no identity declines an identity inequality", src: "entity.assignee ~= current_user.id"},
		{name: "no identity declines a membership", src: "has_current_user(entity.watchers)"},
		{name: "a typed property declines", src: "entity.count ~= 3"},
		{name: "current_user.tool declines", src: "entity.assignee ~= current_user.tool", identity: identity},
		{name: "or declines", src: "entity.status ~= 'a' or related(entity, 'implements')"},
		{name: "an ordered comparison declines", src: "entity.status < 'b'"},
		{name: "not over a comparison declines", src: "not (entity.status == 'a')"},
		{
			name: "no identity declines a negated current_user traversal",
			src:  "not related(entity, 'ownedBy', { id = current_user.id })",
		},
	}
	ev := predicatefns.NewEvaluator(meta)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got, ok := queryplan.LowerCondition(prog, meta, "task", tc.identity)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if !slices.Equal(got.Props, tc.wantProps) {
				t.Errorf("props = %+v, want %+v", got.Props, tc.wantProps)
			}
			var travs []trav
			for _, term := range got.Traversals {
				if len(term.Spec.Refs) != 0 {
					t.Errorf("a lowered traversal is still unbound: %+v", term.Spec)
				}
				travs = append(travs, trav{term.Spec.Path[0], term.Negated})
			}
			if !slices.Equal(travs, tc.wantTrav) {
				t.Errorf("traversals = %v, want %v", travs, tc.wantTrav)
			}
		})
	}
}

// LowerScope is unchanged by the wider lowering: the shapes only
// LowerCondition admits still decline there, because a `visible:` grant
// compiles through LowerScope.
func TestLowerCondition_DoesNotWidenLowerScope(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	for _, src := range []string{
		"entity.status ~= 'done'",
		"not related(entity, 'implements')",
		"has_current_user(entity.watchers)",
		"entity.status == 'a' and entity.status == 'b'",
	} {
		prog, err := ev.CompileWithCurrentUser("task", src)
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		if _, ok := queryplan.LowerCondition(prog, meta, "task", "PERS-JV"); !ok {
			t.Errorf("LowerCondition declined %q", src)
		}
		if _, ok := queryplan.LowerScope(prog, meta, "task", "PERS-JV"); ok {
			t.Errorf("LowerScope lowered %q", src)
		}
	}
}

// The scalar equalities a lowered condition pushes are columns the derived
// index covers (RR-3MMGN9). The other shapes are never index-served: an
// inequality or a membership costs time on a large type, never rows.
func TestLowerCondition_EqualitiesAreIndexColumns(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	src := "entity.status == 'open' and entity.assignee ~= current_user.id and has_current_user(entity.watchers)"
	prog, err := ev.CompileWithCurrentUser("task", src)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := queryplan.LowerCondition(prog, meta, "task", "PERS-JV")
	if !ok {
		t.Fatalf("%q did not lower", src)
	}
	indexed := queryplan.ConditionIndexProperties(prog, meta, []string{"task"})
	for _, p := range got.Props {
		if p.Op == store.PropEqual && p.Scalar && !slices.Contains(indexed, p.Property) {
			t.Errorf("pushes %q, which no derived index covers (%v)", p.Property, indexed)
		}
	}
}

func TestLowerCondition_NilInputs(t *testing.T) {
	meta := conditionMeta()
	prog, err := predicatefns.NewEvaluator(meta).CompileWithCurrentUser("task", "entity.status ~= 'a'")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := queryplan.LowerCondition(nil, meta, "task", ""); ok {
		t.Error("nil program lowered")
	}
	if _, ok := queryplan.LowerCondition(prog, nil, "task", ""); ok {
		t.Error("nil metamodel lowered")
	}
	if _, ok := queryplan.LowerCondition(prog, meta, "", ""); ok {
		t.Error("empty type lowered")
	}
	if _, ok := queryplan.LowerCondition(prog, meta, "undeclared", ""); ok {
		t.Error("an inequality on an undeclared type lowered")
	}
}
