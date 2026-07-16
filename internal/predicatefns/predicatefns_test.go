package predicatefns_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

func TestHostFuncs(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"name":  predicate.StringType,
		"title": predicate.StringType,
		"tags":  predicate.ListType{Elem: predicate.StringType},
		"due":   predicate.DateTypeWithLayout("2006-01-02"),
	}); err != nil {
		t.Fatalf("declare var: %v", err)
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare fns: %v", err)
	}

	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)

	bind := func(name, title string, tags []string, due string) *predicate.Bindings {
		b := predicate.NewBindings()
		elems := make([]predicate.Value, len(tags))
		for i, tg := range tags {
			elems[i] = predicate.NewString(tg)
		}
		dt, _ := time.Parse("2006-01-02", due)
		if err := b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
			"name":  predicate.NewString(name),
			"title": predicate.NewString(title),
			"tags":  predicate.NewList(elems),
			"due":   predicate.NewDate(dt),
		})); err != nil {
			t.Fatalf("set var: %v", err)
		}
		if err := predicatefns.Bind(b, now); err != nil {
			t.Fatalf("bind fns: %v", err)
		}
		return b
	}

	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"glob match", "match(entity.name, '*.md')", true},
		{"glob no-match", "match(entity.name, '*.txt')", false},
		{"regex match", "regex(entity.name, '^RES%-')", false}, // name is README.md
		{"regex match true", "regex(entity.name, 'READ')", true},
		{"fuzzy match close typo", "fuzzy(entity.title, 'authorizaton')", true}, // one-char drop of "authorization"
		{"fuzzy no-match far", "fuzzy(entity.title, 'zzzzzz')", false},
		{"contains present", "contains(entity.tags, 'urgent')", true},
		{"contains absent", "contains(entity.tags, 'later')", false},
		{"today comparison", "entity.due < today()", true}, // due 2026-01-01 < today 2026-07-16
		{"today comparison false", "entity.due > today()", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := predicate.Compile(env, tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got, err := prog.Eval(context.Background(), bind("README.md", "authorization", []string{"urgent", "backend"}, "2026-01-01"))
			if err != nil {
				t.Fatalf("eval %q: %v", tc.src, err)
			}
			b := got.(predicate.Bool)
			if b.Bool() != tc.want {
				t.Errorf("%q = %v, want %v", tc.src, b.Bool(), tc.want)
			}
		})
	}
}
