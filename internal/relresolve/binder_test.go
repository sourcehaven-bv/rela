package relresolve_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func meta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"status": {Type: metamodel.PropertyTypeString}}},
			"person": {Properties: map[string]metamodel.PropertyDef{"name": {Type: metamodel.PropertyTypeString}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"owned-by": {From: []string{"ticket"}, To: []string{"person"}},
			"blocks":   {From: []string{"ticket"}, To: []string{"ticket"}},
		},
	}
}

func compile(t *testing.T, src string) *predicate.Program {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{"id": predicate.StringType}); err != nil {
		t.Fatal(err)
	}
	prog, err := predicate.Compile(env, src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return prog
}

// recorder counts gate and match calls and answers every id in answer.
type recorder struct {
	gates, matches int
	gateErr        error
	answer         map[string]bool
	lastIDs        []string
}

func (r *recorder) gate(_ context.Context, _ string, hop acl.TraversalHop) (*store.RelationPredicate, error) {
	r.gates++
	if r.gateErr != nil {
		return nil, r.gateErr
	}
	return acl.UngatedTraversal(hop)
}

func (r *recorder) match(_ context.Context, _ store.GraphQuery, ids []string) (map[string]bool, error) {
	r.matches++
	r.lastIDs = ids
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = r.answer[id]
	}
	return out, nil
}

func row(id string) predicate.Value {
	return predicate.NewRecord(map[string]predicate.Value{"id": predicate.NewString(id)})
}

func TestNewBinder_RejectsNil(t *testing.T) {
	r := &recorder{}
	cases := []struct {
		name  string
		meta  *metamodel.Metamodel
		gate  relresolve.Gate
		match relresolve.Match
	}{
		{"meta", nil, r.gate, r.match},
		{"gate", meta(), nil, r.match},
		{"match", meta(), r.gate, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := relresolve.NewBinder(tc.meta, tc.gate, tc.match); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestBinder_NoTraversalIsFree(t *testing.T) {
	r := &recorder{}
	b, err := relresolve.NewBinder(meta(), r.gate, r.match)
	if err != nil {
		t.Fatal(err)
	}
	f, err := b.Bind(context.Background(), "ticket", []string{"T-1"}, compile(t, "entity.id == 'T-1'"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if f("T-1") != nil {
		t.Fatal("want a nil traversal func for a program without related()")
	}
	if r.gates != 0 || r.matches != 0 {
		t.Fatalf("gate/match calls = %d/%d, want 0/0", r.gates, r.matches)
	}
}

func TestBinder_OneQueryPerDistinctSpecAcrossPrograms(t *testing.T) {
	r := &recorder{answer: map[string]bool{"T-1": true}}
	b, err := relresolve.NewBinder(meta(), r.gate, r.match)
	if err != nil {
		t.Fatal(err)
	}
	when := compile(t, "related(entity, 'owned-by')")
	then := compile(t, "related(entity, 'owned-by') and not related(entity, 'blocks')")
	f, err := b.Bind(context.Background(), "ticket", []string{"T-2", "T-1", "T-2"}, when, then)
	if err != nil {
		t.Fatal(err)
	}
	if r.gates != 2 || r.matches != 2 {
		t.Fatalf("gate/match calls = %d/%d, want 2/2 (one per distinct traversal)", r.gates, r.matches)
	}
	if len(r.lastIDs) != 2 {
		t.Fatalf("ids sent to the store = %v, want them deduplicated", r.lastIDs)
	}
	spec := when.Traversals()[0]
	for id, want := range map[string]bool{"T-1": true, "T-2": false} {
		got, err := f(id)(row(id), spec)
		if err != nil || got != want {
			t.Fatalf("%s: got (%v, %v), want (%v, nil)", id, got, err, want)
		}
	}
}

func TestBinder_DeniedMatchesNothing(t *testing.T) {
	r := &recorder{gateErr: acl.ErrTraversalDenied, answer: map[string]bool{"T-1": true}}
	b, err := relresolve.NewBinder(meta(), r.gate, r.match)
	if err != nil {
		t.Fatal(err)
	}
	prog := compile(t, "related(entity, 'owned-by')")
	f, err := b.Bind(context.Background(), "ticket", []string{"T-1"}, prog)
	if err != nil {
		t.Fatal(err)
	}
	if r.matches != 0 {
		t.Fatalf("match calls = %d, want 0 for a denied traversal", r.matches)
	}
	got, err := f("T-1")(row("T-1"), prog.Traversals()[0])
	if err != nil || got {
		t.Fatalf("got (%v, %v), want (false, nil)", got, err)
	}
}

func TestBinder_UnsupportedPropagates(t *testing.T) {
	r := &recorder{gateErr: acl.ErrTraversalUnsupported}
	b, err := relresolve.NewBinder(meta(), r.gate, r.match)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Bind(context.Background(), "ticket", []string{"T-1"}, compile(t, "not related(entity, 'owned-by')"))
	if !errors.Is(err, acl.ErrTraversalUnsupported) {
		t.Fatalf("err = %v, want ErrTraversalUnsupported", err)
	}
}

func TestBinder_EmptyCandidatesMakeNoStoreCall(t *testing.T) {
	r := &recorder{}
	b, err := relresolve.NewBinder(meta(), r.gate, r.match)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Bind(context.Background(), "ticket", nil, compile(t, "related(entity, 'owned-by')")); err != nil {
		t.Fatal(err)
	}
	if r.gates != 0 || r.matches != 0 {
		t.Fatalf("gate/match calls = %d/%d, want 0/0", r.gates, r.matches)
	}
}

func TestBinder_UnknownRelationIsAnError(t *testing.T) {
	r := &recorder{}
	b, err := relresolve.NewBinder(meta(), r.gate, r.match)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Bind(context.Background(), "ticket", []string{"T-1"}, compile(t, "related(entity, 'nope')")); err == nil {
		t.Fatal("want an error for an unknown relation")
	}
}
