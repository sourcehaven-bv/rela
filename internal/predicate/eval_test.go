package predicate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// stubBindings builds a Bindings with default host-fn stubs matching
// the env declared in testEnv. Per-case overrides apply on top.
func stubBindings(t *testing.T, vars map[string]predicate.Value, overrides map[string]predicate.Func) *predicate.Bindings {
	t.Helper()
	b := predicate.NewBindings()
	for name, v := range vars {
		if err := b.SetVar(name, v); err != nil {
			t.Fatalf("SetVar %q: %v", name, err)
		}
	}
	defaults := map[string]predicate.Func{
		"has_role": predicate.FuncFunc(func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
			return predicate.NewBool(false), nil
		}),
		"has_relation": predicate.FuncFunc(func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
			return predicate.NewBool(false), nil
		}),
		"count_relations": predicate.FuncFunc(func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
			return predicate.NewNumber(0), nil
		}),
		"is_one_of": predicate.FuncFunc(func(_ context.Context, args []predicate.Value) (predicate.Value, error) {
			if len(args) < 1 {
				return predicate.NewBool(false), nil
			}
			needle := args[0]
			for _, h := range args[1:] {
				if equalValues(needle, h) {
					return predicate.NewBool(true), nil
				}
			}
			return predicate.NewBool(false), nil
		}),
	}
	for name, f := range overrides {
		defaults[name] = f
	}
	for name, f := range defaults {
		if err := b.SetFunc(name, f); err != nil {
			t.Fatalf("SetFunc %q: %v", name, err)
		}
	}
	return b
}

// equalValues mirrors the engine's equality rules for the Value types
// is_one_of needs to compare. Kept in test code only because the
// engine's valuesEqual is unexported; tests of equality semantics
// transitively cover valuesEqual via the accept corpus.
func equalValues(a, b predicate.Value) bool {
	switch av := a.(type) {
	case predicate.String:
		bv, ok := b.(predicate.String)
		return ok && av.String() == bv.String()
	case predicate.Number:
		bv, ok := b.(predicate.Number)
		return ok && av.Float() == bv.Float()
	case predicate.Bool:
		bv, ok := b.(predicate.Bool)
		return ok && av.Bool() == bv.Bool()
	}
	return false
}

func TestProgram_Eval_EndToEnd(t *testing.T) {
	env := testEnv(t)
	ctx := context.Background()

	type scenario struct {
		name           string
		src            string
		varsTrue       map[string]predicate.Value
		varsFalse      map[string]predicate.Value
		overridesTrue  map[string]predicate.Func
		overridesFalse map[string]predicate.Func
	}

	entityWithStatus := func(s string) predicate.Value {
		return predicate.NewRecord(map[string]predicate.Value{
			"status": predicate.NewString(s),
		})
	}

	cases := []scenario{
		{
			name: "1.1 status in progress",
			src:  "is_one_of(entity.status, 'backlog', 'ready', 'planning')",
			varsTrue: map[string]predicate.Value{
				"entity": entityWithStatus("backlog"),
			},
			varsFalse: map[string]predicate.Value{
				"entity": entityWithStatus("in-progress"),
			},
		},
		{
			name: "2.1 owner check",
			src:  "entity.created_by == current_user.id",
			varsTrue: map[string]predicate.Value{
				"entity":       predicate.NewRecord(map[string]predicate.Value{"created_by": predicate.NewString("alice")}),
				"current_user": predicate.NewRecord(map[string]predicate.Value{"id": predicate.NewString("alice")}),
			},
			varsFalse: map[string]predicate.Value{
				"entity":       predicate.NewRecord(map[string]predicate.Value{"created_by": predicate.NewString("alice")}),
				"current_user": predicate.NewRecord(map[string]predicate.Value{"id": predicate.NewString("bob")}),
			},
		},
		{
			name: "3.1 review done gate (composition)",
			src:  "entity.status == 'review' and entity.assignee ~= entity.created_by and entity.effort ~= nil",
			varsTrue: map[string]predicate.Value{
				"entity": predicate.NewRecord(map[string]predicate.Value{
					"status": predicate.NewString("review"), "assignee": predicate.NewString("bob"),
					"created_by": predicate.NewString("alice"), "effort": predicate.NewString("m"),
				}),
			},
			varsFalse: map[string]predicate.Value{
				"entity": predicate.NewRecord(map[string]predicate.Value{
					"status": predicate.NewString("review"), "assignee": predicate.NewString("alice"),
					"created_by": predicate.NewString("alice"), "effort": predicate.NewString("m"),
				}),
			},
		},
		{
			name: "4.2 cardinality cap (number-returning function)",
			src:  "count_relations(entity, 'implements') < 3",
			varsTrue: map[string]predicate.Value{
				"entity": entityWithStatus("ready"),
			},
			varsFalse: map[string]predicate.Value{
				"entity": entityWithStatus("ready"),
			},
			overridesTrue: map[string]predicate.Func{
				"count_relations": predicate.FuncFunc(func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
					return predicate.NewNumber(2), nil
				}),
			},
			overridesFalse: map[string]predicate.Func{
				"count_relations": predicate.FuncFunc(func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
					return predicate.NewNumber(5), nil
				}),
			},
		},
		{
			name: "6.1 env global",
			src:  "env.frozen_for_audit == false",
			varsTrue: map[string]predicate.Value{
				"env": predicate.NewRecord(map[string]predicate.Value{"frozen_for_audit": predicate.NewBool(false)}),
			},
			varsFalse: map[string]predicate.Value{
				"env": predicate.NewRecord(map[string]predicate.Value{"frozen_for_audit": predicate.NewBool(true)}),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := predicate.Compile(env, tc.src)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			runEval := func(t *testing.T, b *predicate.Bindings, want bool) {
				t.Helper()
				v, err := prog.Eval(ctx, b)
				if err != nil {
					t.Fatalf("Eval: %v", err)
				}
				bv, ok := v.(predicate.Bool)
				if !ok {
					t.Fatalf("Eval returned %T, want Bool", v)
				}
				if bv.Bool() != want {
					t.Fatalf("Eval = %v, want %v", bv.Bool(), want)
				}
			}
			runEval(t, stubBindings(t, tc.varsTrue, tc.overridesTrue), true)
			runEval(t, stubBindings(t, tc.varsFalse, tc.overridesFalse), false)
		})
	}
}

func TestProgram_Eval_StepBudget(t *testing.T) {
	env := testEnv(t)
	var b strings.Builder
	b.WriteString("true")
	for range 50 {
		b.WriteString(" and true")
	}
	prog, err := predicate.Compile(env, b.String())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = prog.Eval(context.Background(), predicate.NewBindings(), predicate.WithStepBudget(10))
	if err == nil {
		t.Fatal("expected EvalError on step-budget exhaustion, got nil")
	}
	var ee *predicate.EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
	if !strings.Contains(ee.Reason, "step budget") {
		t.Fatalf("unexpected reason: %s", ee.Reason)
	}
}

func TestEval_EqualitySemantics(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("v", predicate.RecordType{
		"a": predicate.StringType,
		"b": predicate.StringType,
		"n": predicate.NumberType,
		"m": predicate.NumberType,
		"x": predicate.BoolType,
		"y": predicate.BoolType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	ctx := context.Background()
	type tc struct {
		src  string
		vars map[string]predicate.Value
		want bool
	}
	cases := []tc{
		{src: "v.a == v.a", vars: rec("a", "hi"), want: true},
		{src: "v.a == v.b", vars: recAB("hi", "hi"), want: true},
		{src: "v.a == v.b", vars: recAB("hi", "bye"), want: false},
		{src: "v.a == nil", vars: rec("a", "hi"), want: false},
		// Embedded null bytes preserved byte-for-byte.
		{src: "v.a == v.b", vars: recAB("x\x00y", "x\x00y"), want: true},
		{src: "v.a == v.b", vars: recAB("x\x00y", "x\x00z"), want: false},
		// Number equality across lexical forms.
		{src: "v.n == v.m", vars: recNM(1, 1), want: true},
		{src: "v.n == 1.0", vars: rec1num("n", 1), want: true},
		// nil semantics.
		{src: "nil == nil", vars: nil, want: true},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			prog, err := predicate.Compile(env, c.src)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			b := predicate.NewBindings()
			for name, v := range c.vars {
				if setErr := b.SetVar(name, v); setErr != nil {
					t.Fatalf("SetVar: %v", setErr)
				}
			}
			v, err := prog.Eval(ctx, b)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			got := v.(predicate.Bool).Bool()
			if got != c.want {
				t.Fatalf("Eval = %v, want %v", got, c.want)
			}
		})
	}
}

func rec(k, v string) map[string]predicate.Value {
	return map[string]predicate.Value{"v": predicate.NewRecord(map[string]predicate.Value{k: predicate.NewString(v)})}
}
func recAB(va, vb string) map[string]predicate.Value {
	return map[string]predicate.Value{"v": predicate.NewRecord(map[string]predicate.Value{"a": predicate.NewString(va), "b": predicate.NewString(vb)})}
}
func rec1num(k string, v float64) map[string]predicate.Value {
	return map[string]predicate.Value{"v": predicate.NewRecord(map[string]predicate.Value{k: predicate.NewNumber(v)})}
}
func recNM(vn, vm float64) map[string]predicate.Value {
	return map[string]predicate.Value{"v": predicate.NewRecord(map[string]predicate.Value{"n": predicate.NewNumber(vn), "m": predicate.NewNumber(vm)})}
}

// TestProgram_Eval_NilMissingAttr: a declared attr may be missing
// from a binding; the engine treats the missing field as nil so a
// rule can test for absence.
func TestProgram_Eval_NilMissingAttr(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"optional": predicate.StringType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "entity.optional == nil")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	b := predicate.NewBindings()
	_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{}))
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if !v.(predicate.Bool).Bool() {
		t.Fatal("missing attr should compare equal to nil")
	}
}

// Number-roundtrip-via-eval (RR-8VKE (a)): confirm 0xFF compiles AND
// evaluates to 255 (not e.g. 0).
func TestEval_NumberLexicalFormsRoundtrip(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("v", predicate.NumberType); err != nil {
		t.Fatalf("declare: %v", err)
	}
	cases := []struct {
		src      string
		bind     float64
		wantTrue bool
	}{
		{"v == 0xFF", 255, true},
		{"v == 0xFF", 254, false},
		{"v == 1e10", 1e10, true},
		{"v == 1.5e-3", 0.0015, true},
		{"v == 1.5e-3", 0.0016, false},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			prog, err := predicate.Compile(env, c.src)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			b := predicate.NewBindings()
			_ = b.SetVar("v", predicate.NewNumber(c.bind))
			v, err := prog.Eval(context.Background(), b)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			got := v.(predicate.Bool).Bool()
			if got != c.wantTrue {
				t.Fatalf("Eval = %v, want %v", got, c.wantTrue)
			}
		})
	}
}

// TestEval_MissingBinding asserts the eval-time error message when a
// compiled program references a declared variable that the caller
// never bound. Format matters: rule authors hit this when they
// forget to wire up a host-side input.
func TestEval_MissingBinding(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"status": predicate.StringType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "entity.status == 'x'")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = prog.Eval(context.Background(), predicate.NewBindings())
	var ee *predicate.EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
	if !strings.Contains(ee.Reason, `binding "entity" not provided`) {
		t.Fatalf("unexpected reason: %s", ee.Reason)
	}
}

// TestEval_MissingHostFunc asserts the eval-time error message when a
// compiled program references a declared host function that the
// caller never implemented.
func TestEval_MissingHostFunc(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareFunc("ping", predicate.FuncSig{Return: predicate.BoolType}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "ping()")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = prog.Eval(context.Background(), predicate.NewBindings())
	var ee *predicate.EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
	if !strings.Contains(ee.Reason, `host function "ping" not provided`) {
		t.Fatalf("unexpected reason: %s", ee.Reason)
	}
}

// TestEval_HostFuncReturnsWrongType pins the message when a host fn
// returns a value of a different type than declared.
func TestEval_HostFuncReturnsWrongType(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareFunc("wrong", predicate.FuncSig{Return: predicate.BoolType}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "wrong()")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	b := predicate.NewBindings()
	_ = b.SetFunc("wrong", predicate.FuncFunc(
		func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
			return predicate.NewString("not a bool"), nil
		},
	))
	_, err = prog.Eval(context.Background(), b)
	var ee *predicate.EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
	if !strings.Contains(ee.Reason, "declared return bool, got string") {
		t.Fatalf("unexpected reason: %s", ee.Reason)
	}
}

// TestEval_HostFuncReturnsNil pins the message when a host fn returns
// a Go nil interface (caller bug — they should return NewNil()).
func TestEval_HostFuncReturnsNil(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareFunc("oops", predicate.FuncSig{Return: predicate.BoolType}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "oops()")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	b := predicate.NewBindings()
	_ = b.SetFunc("oops", predicate.FuncFunc(
		func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
			// Deliberately returns Go nil to exercise the engine's
			// "use NewNil()" hint; the engine treats this as an
			// EvalError, not a panic.
			return nil, nil //nolint:nilnil // intentional caller bug under test
		},
	))
	_, err = prog.Eval(context.Background(), b)
	var ee *predicate.EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
	if !strings.Contains(ee.Reason, "use NewNil()") {
		t.Fatalf("expected message to hint at NewNil(); got: %s", ee.Reason)
	}
}

// TestEval_HostFuncErrorPropagates pins the message format when a
// host fn returns its own error.
func TestEval_HostFuncErrorPropagates(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareFunc("explode", predicate.FuncSig{Return: predicate.BoolType}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "explode()")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	b := predicate.NewBindings()
	_ = b.SetFunc("explode", predicate.FuncFunc(
		func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
			return nil, errors.New("upstream said no")
		},
	))
	_, err = prog.Eval(context.Background(), b)
	var ee *predicate.EvalError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EvalError, got %T: %v", err, err)
	}
	if !strings.Contains(ee.Reason, "upstream said no") {
		t.Fatalf("host error not propagated: %s", ee.Reason)
	}
	if !strings.Contains(ee.Reason, `host function "explode"`) {
		t.Fatalf("expected host name in message: %s", ee.Reason)
	}
}

// TestEval_ThreadsContext verifies the ctx passed to Eval reaches
// host-function implementations (RR-PCLY).
func TestEval_ThreadsContext(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareFunc("ping", predicate.FuncSig{Return: predicate.BoolType}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "ping()")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")

	var seen string
	b := predicate.NewBindings()
	_ = b.SetFunc("ping", predicate.FuncFunc(
		func(c context.Context, _ []predicate.Value) (predicate.Value, error) {
			if v, ok := c.Value(ctxKey{}).(string); ok {
				seen = v
			}
			return predicate.NewBool(true), nil
		},
	))
	if _, err := prog.Eval(ctx, b); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if seen != "sentinel" {
		t.Fatalf("host fn did not receive caller ctx; seen=%q", seen)
	}
}

// Record.Type() returning RecordType{} is intentional — pin it
// (RR-8VKE (c)).
func TestRecord_TypeIsEmptyRecord(t *testing.T) {
	r := predicate.NewRecord(map[string]predicate.Value{
		"status": predicate.NewString("x"),
	})
	if _, ok := r.Type().(predicate.RecordType); !ok {
		t.Fatalf("Record.Type() did not return RecordType, got %T", r.Type())
	}
}
