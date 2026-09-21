package predicate

import (
	"context"
	"strings"
	"testing"
)

func traversalEnv(t *testing.T) *Env {
	t.Helper()
	env := NewEnv()
	if err := env.DeclareVar("entity", RecordType{"status": StringType}); err != nil {
		t.Fatal(err)
	}
	return env
}

// The form must compile to a traversal the lowering can read back exactly —
// path, type ascription and property constraints — because that description
// IS what gets pushed into the store.
func TestRelated_CompilesToAnInspectableSpec(t *testing.T) {
	prog, err := Compile(traversalEnv(t),
		`related(entity, 'caused-by', { type = 'ticket', status = 'done' })`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	specs := prog.Traversals()
	if len(specs) != 1 {
		t.Fatalf("expected 1 traversal, got %d", len(specs))
	}
	s := specs[0]
	if len(s.Path) != 1 || s.Path[0] != "caused-by" {
		t.Errorf("path = %v", s.Path)
	}
	if s.EntityType != "ticket" {
		t.Errorf("entity type = %q", s.EntityType)
	}
	if got := s.PropNames(); len(got) != 1 || got[0] != "status" {
		t.Fatalf("prop names = %v", got)
	}
	if v, ok := s.Props["status"].(String); !ok || v.String() != "done" {
		t.Errorf("status prop = %#v", s.Props["status"])
	}
}

// A traversal must NOT taint SQLPortable. That is the whole reason the form is
// compiled statically instead of being a host function: one non-portable node
// disables pushdown for every other clause in the same condition.
func TestRelated_StaysSQLPortable(t *testing.T) {
	prog, err := Compile(traversalEnv(t),
		`entity.status == 'open' and related(entity, 'caused-by', { type = 'ticket' })`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !prog.SQLPortable() {
		t.Fatal("a traversal must stay SQL-portable, or it defeats its own purpose")
	}
	// It must also not be mistaken for a host function.
	for _, fn := range prog.Functions() {
		if fn == FuncRelated {
			t.Fatalf("%s must not be reported as a host function", FuncRelated)
		}
	}
}

func TestRelated_ChainedPath(t *testing.T) {
	prog, err := Compile(traversalEnv(t),
		`related(entity, { 'rel-x', 'rel-y' }, { type = 'C' })`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := prog.Traversals()[0].Path
	if len(got) != 2 || got[0] != "rel-x" || got[1] != "rel-y" {
		t.Fatalf("path = %v", got)
	}
}

func TestRelated_CompileErrors(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"no args", `related()`, "expected 2 or 3 args"},
		{"too many args", `related(entity, 'a', {}, {})`, "expected 2 or 3 args"},
		{"subject not a record", `related('nope', 'a')`, "must be an entity record"},
		{"empty relation", `related(entity, '')`, "must be non-empty"},
		{"empty path list", `related(entity, {})`, "at least one relation type"},
		{"keyed path list", `related(entity, {a='b'})`, "plain list"},
		{"non-string path", `related(entity, 42)`, "must be a string or a list"},
		{"non-table constraints", `related(entity, 'a', 'b')`, "must be a table literal"},
		{"computed constraint value", `related(entity, 'a', {status=entity.status})`, "must be a constant"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(traversalEnv(t), tc.src)
			if err == nil {
				t.Fatalf("expected a compile error for %q", tc.src)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

// An unbound traversal must FAIL rather than answer false: a false would
// present an unanswerable traversal as a legitimate "no match", which is the
// wrong direction for a condition gating access or an automation.
func TestRelated_UnboundResolverFailsRatherThanAnsweringFalse(t *testing.T) {
	prog, err := Compile(traversalEnv(t), `related(entity, 'caused-by')`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	b := NewBindings()
	if err := b.SetVar("entity", NewRecord(map[string]Value{"status": NewString("open")})); err != nil {
		t.Fatal(err)
	}
	if _, err := prog.Eval(context.Background(), b); err == nil {
		t.Fatal("an unbound traversal resolver must fail the Eval")
	}
}

func TestRelated_EvaluatesThroughTheBoundResolver(t *testing.T) {
	prog, err := Compile(traversalEnv(t), `related(entity, 'caused-by', { type = 'ticket' })`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var seen TraversalSpec
	b := NewBindings()
	if setErr := b.SetVar("entity", NewRecord(map[string]Value{"status": NewString("open")})); setErr != nil {
		t.Fatal(setErr)
	}
	b.SetTraversal(func(_ Value, spec TraversalSpec) (bool, error) {
		seen = spec
		return true, nil
	})
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got, ok := v.(Bool); !ok || !got.Bool() {
		t.Fatalf("expected true, got %#v", v)
	}
	if seen.EntityType != "ticket" {
		t.Errorf("resolver saw spec %+v", seen)
	}
}
