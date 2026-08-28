package predicate_test

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

func valueEnv(t *testing.T) *predicate.Env {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"a": predicate.IntType, "b": predicate.IntType,
		"first": predicate.StringType, "last": predicate.StringType,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.DeclareFunc("portable", predicate.FuncSig{Return: predicate.StringType, SQLPortable: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.DeclareFunc("host_only", predicate.FuncSig{Return: predicate.StringType}); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestCompileValue_ArithmeticConcatDependenciesAndPortability(t *testing.T) {
	env := valueEnv(t)
	prog, err := predicate.CompileValue(env, "entity.a * entity.b + 2", predicate.ValueProfile(predicate.IntType))
	if err != nil {
		t.Fatalf("CompileValue: %v", err)
	}
	if got, want := prog.Attributes("entity"), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("attributes = %v, want %v", got, want)
	}
	if !prog.SQLPortable() {
		t.Fatal("arithmetic/property program should be SQL-portable")
	}
	b := predicate.NewBindings()
	_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
		"a": predicate.NewInt(4), "b": predicate.NewInt(5),
	}))
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got := v.(predicate.Int).Int64(); got != 22 {
		t.Fatalf("result = %d, want 22", got)
	}

	concat, err := predicate.CompileValue(env, "entity.first .. ' ' .. entity.last", predicate.ValueProfile(predicate.StringType))
	if err != nil {
		t.Fatalf("CompileValue concat: %v", err)
	}
	b = predicate.NewBindings()
	_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
		"first": predicate.NewString("Ada"), "last": predicate.NewString("Lovelace"),
	}))
	v, err = concat.Eval(context.Background(), b)
	if err != nil || v.(predicate.String).String() != "Ada Lovelace" {
		t.Fatalf("concat = %v, %v", v, err)
	}
}

func TestCompileValue_ProfileAndErrors(t *testing.T) {
	env := valueEnv(t)
	if _, err := predicate.Compile(env, "entity.a + 1 == 2"); err == nil || !strings.Contains(err.Error(), "arithmetic operators are not allowed") {
		t.Fatalf("legacy Compile arithmetic error = %v", err)
	}
	if _, err := predicate.CompileValue(env, "entity.first", predicate.ValueProfile(predicate.IntType)); err == nil || !strings.Contains(err.Error(), "must be int") {
		t.Fatalf("result mismatch error = %v", err)
	}
	profile := predicate.ValueProfile(predicate.StringType)
	profile.RequireSQLPortable = true
	if _, err := predicate.CompileValue(env, "host_only()", profile); err == nil || !strings.Contains(err.Error(), "not SQL-portable") {
		t.Fatalf("portability error = %v", err)
	}
	if p, err := predicate.CompileValue(env, "portable()", profile); err != nil || !p.SQLPortable() {
		t.Fatalf("portable compile = %v, %v", p, err)
	}
}

func TestCompileValue_CheckedIntegerErrors(t *testing.T) {
	env := valueEnv(t)
	prog, err := predicate.CompileValue(env, "entity.a + entity.b", predicate.ValueProfile(predicate.IntType))
	if err != nil {
		t.Fatal(err)
	}
	b := predicate.NewBindings()
	_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
		"a": predicate.NewInt(math.MaxInt64), "b": predicate.NewInt(1),
	}))
	if _, err := prog.Eval(context.Background(), b); err == nil || !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("overflow error = %v", err)
	}
}

func TestCompileValue_NumberDivisionAndModuloByZero(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"a": predicate.NumberType,
		"b": predicate.NumberType,
	}); err != nil {
		t.Fatal(err)
	}
	bindings := predicate.NewBindings()
	_ = bindings.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
		"a": predicate.NewNumber(4),
		"b": predicate.NewNumber(0),
	}))
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"entity.a / entity.b", "division by zero"},
		{"entity.a % entity.b", "modulo by zero"},
	} {
		prog, err := predicate.CompileValue(env, tc.expr, predicate.ValueProfile(predicate.NumberType))
		if err != nil {
			t.Fatalf("CompileValue(%q): %v", tc.expr, err)
		}
		if _, err := prog.Eval(context.Background(), bindings); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Eval(%q) error = %v, want %q", tc.expr, err, tc.want)
		}
	}
}
