package predicate_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

func TestCompileAll_ReportsAllSourceErrors(t *testing.T) {
	env := testEnv(t)
	sources := []predicate.NamedSource{
		{Name: "ok1", Source: "entity.status == 'open'"},
		{Name: "bad-unknown-var", Source: "missing_var == 'x'"},
		{Name: "ok2", Source: "has_role(current_user, 'admin')"},
		{Name: "bad-function-literal", Source: "function() return true end"},
		{Name: "ok3", Source: "count_relations(entity, 'x') < 5"},
	}

	progs, issues := predicate.CompileAll(env, sources)

	if len(progs) != len(sources) {
		t.Fatalf("CompileAll returned %d programs, want %d", len(progs), len(sources))
	}
	if len(issues) != 2 {
		t.Fatalf("CompileAll returned %d issues, want 2: %+v", len(issues), issues)
	}
	if issues[0].Name != "bad-unknown-var" {
		t.Errorf("issues[0].Name = %q, want bad-unknown-var", issues[0].Name)
	}
	if issues[1].Name != "bad-function-literal" {
		t.Errorf("issues[1].Name = %q, want bad-function-literal", issues[1].Name)
	}
	for _, iss := range issues {
		if iss.Err == nil {
			t.Errorf("issue %q has nil err", iss.Name)
		}
	}
	// Failed sources have a nil entry; successful ones have a non-nil
	// program at the matching index.
	if progs[0] == nil {
		t.Error("progs[0] (ok1) should be non-nil")
	}
	if progs[1] != nil {
		t.Error("progs[1] (bad-unknown-var) should be nil")
	}
	if progs[2] == nil {
		t.Error("progs[2] (ok2) should be non-nil")
	}
	if progs[3] != nil {
		t.Error("progs[3] (bad-function-literal) should be nil")
	}
	if progs[4] == nil {
		t.Error("progs[4] (ok3) should be non-nil")
	}
}

func TestCompileAll_AllClean(t *testing.T) {
	env := testEnv(t)
	sources := []predicate.NamedSource{
		{Name: "a", Source: "true"},
		{Name: "b", Source: "false"},
		{Name: "c", Source: "entity.status == 'x'"},
	}
	progs, issues := predicate.CompileAll(env, sources)
	if len(issues) != 0 {
		t.Fatalf("CompileAll returned %d issues on a clean batch, want 0: %+v", len(issues), issues)
	}
	for i, p := range progs {
		if p == nil {
			t.Errorf("progs[%d] (%s) should be non-nil", i, sources[i].Name)
		}
	}
}
