package cli

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestHasWorld covers the metamodelReader adapter's world lookup — the declaration lookup the ACL audit's B10 check
// consumes. The implicit default world is NOT declared and must report
// false — callers that accept it special-case the name themselves, which
// is what keeps "is this world declared" a different question from "is
// this world usable".
func TestHasWorld(t *testing.T) {
	t.Parallel()
	m := &metamodel.Metamodel{Worlds: map[string]metamodel.WorldDef{
		"published": {Select: []string{"published"}, Otherwise: metamodel.OtherwiseExclude},
	}}
	tests := []struct {
		name  string
		world string
		want  bool
	}{
		{"declared world", "published", true},
		{"undeclared world", "editorial", false},
		{"the implicit default world is not DECLARED", "default", false},
		{
			// The loader rejects a declared world whose name case-folds to
			// "default", so no declared world can shadow the implicit one
			// under any spelling — but this lookup is case-SENSITIVE, so a
			// case variant is simply undeclared. B10 gives it its own
			// message because the ordinary "go declare it" fix is one the
			// loader would refuse.
			name: "a case variant of default is undeclared", world: "Default", want: false,
		},
		{"empty name", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := (&metamodelReader{m: m}).HasWorld(tc.world); got != tc.want {
				t.Errorf("HasWorld(%q) = %v, want %v", tc.world, got, tc.want)
			}
		})
	}
	t.Run("nil metamodel", func(t *testing.T) {
		t.Parallel()
		var nilM *metamodel.Metamodel
		if (&metamodelReader{m: nilM}).HasWorld("published") {
			t.Error("a nil metamodel declares nothing")
		}
	})
}

// TestHasPointer covers the adapter's content-state lookup — the content-state lookup B11 consumes.
//
// The ALIAS case is the load-bearing one: HasPointer's doc claims it
// resolves aliases through GetEntityDef, so a grant written against a type
// alias must answer about the canonical type. Nothing else in the tree
// exercises that claim — the audit's own tests use a fake whose HasPointer
// has no alias handling at all.
func TestHasPointer(t *testing.T) {
	t.Parallel()
	m := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"page": {
			Aliases:  []string{"pg"},
			Pointers: map[string]metamodel.PointerDef{"draft": {Default: true}, "published": {}},
		},
		"ticket": {}, // no pointers block at all
	}}
	// The alias map is built at load, not from the literal above, so a
	// hand-constructed metamodel must initialize it or ResolveAlias is a
	// no-op and the alias case below would pass vacuously.
	m.InitAliases()
	tests := []struct {
		name            string
		entityType, ptr string
		want            bool
	}{
		{"declared pointer", "page", "draft", true},
		{"second declared pointer", "page", "published", true},
		{"undeclared pointer on a pointered type", "page", "review", false},
		{"ALIAS resolves to the canonical type", "pg", "draft", true},
		{"alias with an undeclared pointer", "pg", "review", false},
		{"a type with no pointers block declares none", "ticket", "draft", false},
		{"the empty pointer is not a declaration", "page", "", false},
		{"undeclared type", "nosuchtype", "draft", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := (&metamodelReader{m: m}).HasPointer(tc.entityType, tc.ptr); got != tc.want {
				t.Errorf("HasPointer(%q, %q) = %v, want %v",
					tc.entityType, tc.ptr, got, tc.want)
			}
		})
	}
	t.Run("nil metamodel", func(t *testing.T) {
		t.Parallel()
		var nilM *metamodel.Metamodel
		if (&metamodelReader{m: nilM}).HasPointer("page", "draft") {
			t.Error("a nil metamodel declares nothing")
		}
	})
}
