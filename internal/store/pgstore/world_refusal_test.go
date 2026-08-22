//go:build postgres

package pgstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// These pin pgstore's transitional refusal of world-scoped queries
// (TKT-WAV8XP, decision Q8). They are pure-unit — the refusal happens
// before any SQL is built — so they are not gated on
// RELA_TEST_DATABASE_URL.
//
// They live HERE and not in storetest deliberately (RR-MNOBJK). The
// shared conformance suite states ONE contract for all backends; asking
// it to accept correct rows from fs/mem and a refusal from pg would
// defeat the point of having it. fs/mem opt into RunWorldTests via
// Capabilities.Worlds in PR-B; PR-C implements the SQL pushdown, sets
// pg's behavior to match, deletes that flag, and deletes this file.

func worldScope(t *testing.T) store.WorldScope {
	t.Helper()
	p, err := entity.ParsePointer("published")
	if err != nil {
		t.Fatal(err)
	}
	return store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: []entity.Pointer{p}, Fallback: store.FallbackExclude},
	})
}

// TestCheckQueryScope_RefusesNonDefaultWorld pins that the refusal is
// LOUD. The alternative considered and rejected was a naive path
// resolving primes by filtering rows in Go: that would pass the
// conformance suite while doing the per-row scan the design forbids, in
// the one backend that has scale. An error names the gap; a slow correct
// answer hides it.
func TestCheckQueryScope_RefusesNonDefaultWorld(t *testing.T) {
	err := checkQueryScope(store.EntityQuery{Type: "page", World: worldScope(t)})
	if err == nil {
		t.Fatal("pgstore must refuse a world-scoped query until PR-C")
	}
	// The message must name the ticket and the PR that removes it, or the
	// next reader cannot tell a deliberate gap from a missing feature.
	for _, want := range []string{"TKT-WAV8XP", "PR-C"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q, got: %v", want, err)
		}
	}
}

// TestCheckQueryScope_DefaultWorldIsUnaffected is the load-bearing
// negative: the zero WorldScope is the DEFAULT world, not "no world", so
// a refusal keyed on the wrong condition would reject every existing
// query in the codebase.
func TestCheckQueryScope_DefaultWorldIsUnaffected(t *testing.T) {
	for _, q := range []store.EntityQuery{
		{},
		{Type: "page"},
		{Type: "page", AllStates: true},
		{Type: "page", World: store.DefaultWorld()},
	} {
		if err := checkQueryScope(q); err != nil {
			t.Errorf("checkQueryScope(%+v) = %v, want nil", q, err)
		}
	}
}

// TestCheckQueryScope_AllStatesAndWorldConflict pins that pgstore
// inherits the shared contradiction rule (decision Q3) rather than
// reimplementing it. AllStates is raw storage truth and a world resolves
// each entity to one state; honoring both is impossible.
func TestCheckQueryScope_AllStatesAndWorldConflict(t *testing.T) {
	err := checkQueryScope(store.EntityQuery{
		Type: "page", AllStates: true, World: worldScope(t),
	})
	if !errors.Is(err, store.ErrInvalidQuery) {
		t.Fatalf("want ErrInvalidQuery, got %v", err)
	}
}

// TestCheckGraphQueryScope_RefusesNonDefaultWorld covers the second
// query type. The world must reach BOTH or the list path and the ACL
// pushdown path diverge — and they diverge for the AllowAll principal
// specifically, who takes the EntityQuery branch (F1/F5).
func TestCheckGraphQueryScope_RefusesNonDefaultWorld(t *testing.T) {
	if err := checkGraphQueryScope(store.GraphQuery{EntityType: "page"}); err != nil {
		t.Errorf("the default world must be unaffected, got %v", err)
	}
	err := checkGraphQueryScope(store.GraphQuery{EntityType: "page", World: worldScope(t)})
	if err == nil {
		t.Fatal("pgstore must refuse a world-scoped graph query until PR-C")
	}
}
