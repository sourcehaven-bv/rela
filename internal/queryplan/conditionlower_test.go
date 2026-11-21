package queryplan_test

import (
	"errors"
	"slices"
	"strings"
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
		{name: "current_user.tool declines", src: "entity.assignee ~= current_user.tool", identity: identity},
		{
			name: "an ordered string comparison is a prop", src: "entity.status < 'b' and 3 <= entity.count",
			wantOK: true,
			wantProps: []store.PropPredicate{
				{Property: "status", Op: store.PropLess, Value: "b", Kind: store.OrderString},
				{Property: "count", Op: store.PropGreaterEqual, Value: "3", Kind: store.OrderInteger},
			},
		},
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

// A condition that is not a plain conjunction keeps its plain conjuncts in
// Props and Traversals and puts the rest in Where (TKT-8HY05WB).
func TestLowerCondition_Trees(t *testing.T) {
	meta := conditionMeta()
	tests := []struct {
		src, props, where string
	}{
		{src: "entity.status ~= 'a' or related(entity, 'implements')",
			where: "or(status~=a, rel(implements))"},
		{src: "not (entity.status == 'a')", where: "not(status=a)"},
		{src: "entity.count ~= 3", where: "not(and(count>=3#int, count<=3#int))"},
		{src: "entity.count == 3", props: "count>=3#int count<=3#int"},
		{src: "entity.status == 'x' and (entity.count > 1 or not related(entity, 'implements'))",
			props: "status=x", where: "or(count>1#int, not(rel(implements)))"},
		{src: "true and entity.status == 'x'", props: "status=x"},
		{src: "false or entity.status == 'x'", where: "or(or(), status=x)"},
		{src: "entity.status >= ''", props: "status>=#str"},
	}
	ev := predicatefns.NewEvaluator(meta)
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, ok := queryplan.LowerCondition(prog, meta, "task", "")
			if !ok {
				t.Fatal("did not lower")
			}
			var props []string
			for _, p := range got.Props {
				props = append(props, renderProp(p))
			}
			if g := strings.Join(props, " "); g != tc.props {
				t.Errorf("props = %q, want %q", g, tc.props)
			}
			where := ""
			if got.Where != nil {
				where = renderLowered(*got.Where)
			}
			if where != tc.where {
				t.Errorf("where = %q, want %q", where, tc.where)
			}
		})
	}
}

// Shapes the store cannot read as Go does decline.
func TestLowerCondition_TreeDeclines(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	deep := "entity.status == 'a'"
	for range store.MaxCondDepth {
		deep = "not (" + deep + ")"
	}
	// Balanced, so the compiler's own nesting limit does not refuse it
	// first.
	var balanced func(n int) string
	balanced = func(n int) string {
		if n == 1 {
			return "entity.status == 'a'"
		}
		return "(" + balanced(n/2) + " or " + balanced(n-n/2) + ")"
	}
	for _, src := range []string{
		"entity.due < '2026-01-01'",
		"entity.price > 3",
		"entity.status == entity.assignee",
		"entity.count > entity.count",
		deep,
		balanced(store.MaxCondLeaves + 1),
	} {
		t.Run(src[:min(len(src), 40)], func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if _, ok := queryplan.LowerCondition(prog, meta, "task", ""); ok {
				t.Error("lowered")
			}
		})
	}
}

func renderProp(p store.PropPredicate) string {
	op := map[store.PropOp]string{
		store.PropEqual: "=", store.PropNotEqualOrEmpty: "~=", store.PropLess: "<", store.PropLessEqual: "<=",
		store.PropGreater: ">", store.PropGreaterEqual: ">=",
	}[p.Op]
	kind := map[store.OrderKind]string{store.OrderText: "", store.OrderString: "#str", store.OrderInteger: "#int"}[p.Kind]
	return p.Property + op + p.Value + kind
}

func renderLowered(c queryplan.LoweredCond) string {
	var args []string
	for _, a := range c.Args {
		args = append(args, renderLowered(a))
	}
	switch c.Op {
	case store.CondProp:
		return renderProp(c.Prop)
	case store.CondRelated:
		return "rel(" + strings.Join(c.Traversal.Path, ".") + ")"
	case store.CondAnd:
		return "and(" + strings.Join(args, ", ") + ")"
	case store.CondOr:
		return "or(" + strings.Join(args, ", ") + ")"
	default:
		return "not(" + strings.Join(args, ", ") + ")"
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

// A refusal names the construct that does not lower and why, so a caller
// can rewrite the condition (TKT-9BKVQ9I).
func TestLowerConditionExplained_NamesTheConstruct(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	deep := "entity.status == 'a'"
	for range store.MaxCondDepth {
		deep = "not (" + deep + ")"
	}
	for _, tc := range []struct{ src, construct, reason string }{
		{"entity.due < '2026-01-01'", "entity.due < 2026-01-01", "cannot answer"},
		{"entity.price > 3", "entity.price > 3", "cannot answer"},
		{"entity.status == entity.assignee", "entity.status == entity.assignee", "cannot answer"},
		{"entity.status == 'a' or entity.count > entity.count", "entity.count > entity.count", "cannot answer"},
		{"entity.assignee ~= current_user.tool", "current_user.tool", "current_user.id"},
		{"entity.status == ''", `entity.status == ""`, ""},
		{deep, "condition", "too large"},
	} {
		t.Run(tc.src[:min(len(tc.src), 40)], func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			_, err = queryplan.LowerConditionExplained(prog, meta, "task", "PERS-1")
			var unsupported *queryplan.UnsupportedConditionError
			if !errors.As(err, &unsupported) {
				t.Fatalf("err = %v, want an UnsupportedConditionError", err)
			}
			if !strings.Contains(unsupported.Construct, tc.construct) || !strings.Contains(unsupported.Reason, tc.reason) {
				t.Errorf("got %q, want the construct %q and a reason with %q", err, tc.construct, tc.reason)
			}
		})
	}
}
