package predicate

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

// ConditionTreeExplained names the innermost sub-expression it refuses, as
// source text (TKT-9BKVQ9I).
func TestConditionTreeExplained(t *testing.T) {
	env := prefilterEnv(t)
	for _, tc := range []struct{ src, want string }{
		{"entity.status == 'a'", ""},
		{"entity.status == 'a' or entity.status == entity.assignee", "entity.status == entity.assignee"},
		{"not (entity.count > entity.count)", "entity.count > entity.count"},
		{"entity.status == 'a' and is_reporter(entity.assignee)", `is_reporter(entity.assignee)`},
		{"entity.done", "entity.done"},
		{"has_current_user(entity.tags) or any_int(entity.scores)", "any_int(entity.scores)"},
		{"entity.count < -1", ""},
	} {
		t.Run(tc.src, func(t *testing.T) {
			prog, err := Compile(env, tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			_, got := prog.ConditionTreeExplained(fullSpec)
			if got != tc.want {
				t.Errorf("unsupported = %q, want %q", got, tc.want)
			}
			if _, ok := prog.ConditionTree(fullSpec); ok != (tc.want == "") {
				t.Errorf("ConditionTree ok = %v", ok)
			}
		})
	}
	if _, got := (*Program)(nil).ConditionTreeExplained(fullSpec); got != "program" {
		t.Errorf("nil program: %q", got)
	}
}

func TestRender(t *testing.T) {
	env := prefilterEnv(t)
	for _, tc := range []struct{ src, want string }{
		{"entity.status == 'a' and not (entity.count >= 3)", `(entity.status == "a") and not (entity.count >= 3)`},
		{"is_reporter(entity.assignee) or entity.done", "is_reporter(entity.assignee) or entity.done"},
	} {
		prog, err := Compile(env, tc.src)
		if err != nil {
			t.Fatalf("compile %s: %v", tc.src, err)
		}
		if got := render(prog.root); got != tc.want {
			t.Errorf("render(%s) = %q, want %q", tc.src, got, tc.want)
		}
	}
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for v, want := range map[Value]string{
		NewString("x"): `"x"`, Nil{}: "nil", NewBool(true): "true", NewInt(-3): "-3",
		NewNumber(1.5): "1.5", NewDate(day): "2026-01-02", NewDecimal(big.NewRat(5, 2)): "2.5",
	} {
		if got := renderValue(v); got != want {
			t.Errorf("renderValue(%v) = %q, want %q", v, got, want)
		}
	}
	var b strings.Builder
	renderTable(&b, map[string]Value{"b": NewInt(1), "a": NewString("x")})
	if got := b.String(); got != `{ a = "x", b = 1 }` {
		t.Errorf("renderTable = %q", got)
	}
}
