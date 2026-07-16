package predicate_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// TestTypedComparison_IntAndDate is the RR-A3EZR gate: it proves that an
// int/date-typed record attribute compared against a bare string/number
// LITERAL coerces the literal to the attribute's type at compile time,
// then compares correctly at eval — with eval doing no parsing.
func TestTypedComparison_IntAndDate(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"count": predicate.IntType,
		"due":   predicate.DateTypeWithLayout("2006-01-02"),
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}

	mustDate := func(s string) predicate.Value {
		tm, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("parse test date %q: %v", s, err)
		}
		return predicate.NewDate(tm)
	}

	bind := func(count int64, due string) *predicate.Bindings {
		b := predicate.NewBindings()
		if err := b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
			"count": predicate.NewInt(count),
			"due":   mustDate(due),
		})); err != nil {
			t.Fatalf("bind: %v", err)
		}
		return b
	}

	tests := []struct {
		name  string
		src   string
		count int64
		due   string
		want  bool
	}{
		// Integer ordering is numeric, not lexicographic ("10" > "9").
		{"int gt true", "entity.count > 9", 10, "2026-01-01", true},
		{"int gt false", "entity.count > 9", 8, "2026-01-01", false},
		{"int lexico-would-be-wrong", "entity.count > 9", 100, "2026-01-01", true},
		{"int eq", "entity.count == 42", 42, "2026-01-01", true},
		{"int ne", "entity.count ~= 42", 7, "2026-01-01", true},
		// Date ordering is instant-granular against the literal.
		{"date lt true", "entity.due < '2026-02-01'", 0, "2026-01-15", true},
		{"date lt false", "entity.due < '2026-02-01'", 0, "2026-03-01", false},
		{"date gte boundary", "entity.due >= '2026-02-01'", 0, "2026-02-01", true},
		{"date eq", "entity.due == '2026-02-01'", 0, "2026-02-01", true},
		// Composition the old --where syntax could not express.
		{"or across types", "entity.count > 100 or entity.due < '2026-02-01'", 5, "2026-01-01", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := predicate.Compile(env, tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got, err := prog.Eval(context.Background(), bind(tc.count, tc.due))
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			b, ok := got.(predicate.Bool)
			if !ok {
				t.Fatalf("result type = %T, want Bool", got)
			}
			if b.Bool() != tc.want {
				t.Errorf("%q with count=%d due=%s = %v, want %v", tc.src, tc.count, tc.due, b.Bool(), tc.want)
			}
		})
	}
}

// TestTypedComparison_CoercionErrors pins the compile-time rejections.
func TestTypedComparison_CoercionErrors(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"count": predicate.IntType,
		"due":   predicate.DateTypeWithLayout("2006-01-02"),
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}

	tests := []struct {
		name string
		src  string
	}{
		{"malformed date literal", "entity.due < 'not-a-date'"},
		{"non-integer literal on int field", "entity.count > 1.5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := predicate.Compile(env, tc.src); err == nil {
				t.Errorf("compile %q: expected error, got nil", tc.src)
			}
		})
	}
}
