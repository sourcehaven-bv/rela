package predicatefns_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// TestEntityRecordType_EndToEnd proves the metamodel->Env adapter yields
// a RecordType whose typed fields drive correct compile-time coercion:
// an integer field compares numerically and a date field compares
// instant-granular against a bare literal — with no metamodel reference
// at Eval.
func TestEntityRecordType_EndToEnd(t *testing.T) {
	def := &metamodel.EntityDef{
		Properties: map[string]metamodel.PropertyDef{
			"count":  {Type: metamodel.PropertyTypeInteger},
			"due":    {Type: metamodel.PropertyTypeDate},
			"name":   {Type: metamodel.PropertyTypeString},
			"tags":   {Type: metamodel.PropertyTypeString, List: true},
			"active": {Type: metamodel.PropertyTypeBoolean},
		},
	}

	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicatefns.EntityRecordType(def)); err != nil {
		t.Fatalf("declare: %v", err)
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare fns: %v", err)
	}

	// Integer numeric ordering (100 > 9 must be true, not lexicographic)
	// and date coercion of a bare string literal against the DateType field.
	prog, err := predicate.Compile(env,
		"entity.count > 9 and entity.due < '2026-06-01' and contains(entity.tags, 'urgent')")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	b := predicate.NewBindings()
	due, _ := time.Parse("2006-01-02", "2026-01-15")
	if setErr := b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
		"count":  predicate.NewInt(100),
		"due":    predicate.NewDate(due),
		"name":   predicate.NewString("x"),
		"tags":   predicate.NewList([]predicate.Value{predicate.NewString("urgent")}),
		"active": predicate.NewBool(true),
	})); setErr != nil {
		t.Fatalf("bind: %v", setErr)
	}
	if bindErr := predicatefns.Bind(b, time.Now()); bindErr != nil {
		t.Fatalf("bind fns: %v", bindErr)
	}

	got, err := prog.Eval(context.Background(), b)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if !got.(predicate.Bool).Bool() {
		t.Errorf("expected true (count=100>9, due<2026-06-01, tags contains urgent)")
	}
}
