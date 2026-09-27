package scopes_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/scopes"
)

func parse(t *testing.T, yaml string) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return m
}

const scopedSchema = `version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties:
      status: {type: string}
      owner: {type: string}
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
      actief: "entity.status ~= 'gereed'"
      mijn: "is_current_user(entity.owner)"
  notitie:
    label: Notitie
    id_prefix: NOTE
    properties: {status: {type: string}}
`

func TestCompile_ResolvesDeclaredScopes(t *testing.T) {
	c, err := scopes.Compile(parse(t, scopedSchema))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, name := range []string{"default", "actief", "mijn"} {
		if _, ok := c.Lookup("taak", name); !ok {
			t.Errorf("Lookup(taak, %q) not found", name)
		}
	}
	if got, want := strings.Join(c.Names("taak"), ","), "actief,default,mijn"; got != want {
		t.Errorf("Names(taak) = %q, want %q", got, want)
	}
	if got := c.Names("notitie"); len(got) != 0 {
		t.Errorf("Names(notitie) = %v, want empty", got)
	}
}

// TestLookup_UnknownFailsClosed pins the rule Compiled.Lookup exists to
// enforce: an unknown name is ok=false, never a silent nil program. A
// substituted nil would mean "no predicate", turning a typo into "show
// everything" — the opposite of what a default that hides archived rows asks.
func TestLookup_UnknownFailsClosed(t *testing.T) {
	c, err := scopes.Compile(parse(t, scopedSchema))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, ok := c.Lookup("taak", "typo"); ok {
		t.Error("Lookup(taak, typo) reported ok; an unknown scope must fail closed")
	}
	// A scope declared on ANOTHER type must not resolve here: scopes are
	// compiled per type and the key includes it.
	if _, ok := c.Lookup("notitie", "actief"); ok {
		t.Error("Lookup(notitie, actief) resolved a scope declared on taak")
	}
}

// TestLookup_AllIsImplicit pins that `all` resolves everywhere, including for
// a type declaring no scopes and through a zero-value Compiled. It is how a
// view withdraws the default, so it must never be the thing that is missing.
func TestLookup_AllIsImplicit(t *testing.T) {
	c, err := scopes.Compile(parse(t, scopedSchema))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, typeName := range []string{"taak", "notitie", "onbekend"} {
		prog, ok := c.Lookup(typeName, metamodel.AllQueryScopeName)
		if !ok {
			t.Errorf("Lookup(%s, all) not ok", typeName)
		}
		if prog != nil {
			t.Errorf("Lookup(%s, all) returned a program; all means no predicate", typeName)
		}
	}
	var zero scopes.Compiled
	if _, ok := zero.Lookup("taak", metamodel.AllQueryScopeName); !ok {
		t.Error("zero-value Compiled did not resolve all")
	}
	if _, ok := zero.Lookup("taak", "actief"); ok {
		t.Error("zero-value Compiled resolved a declared scope")
	}
}

func TestDefault(t *testing.T) {
	c, err := scopes.Compile(parse(t, scopedSchema))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, ok := c.Default("taak"); !ok {
		t.Error("Default(taak) not found")
	}
	if _, ok := c.Default("notitie"); ok {
		t.Error("Default(notitie) found; the type declares none")
	}
}

// TestCompile_ReportsEveryProblem pins collect-then-report: an operator
// fixing a schema should see every broken scope, not the first.
func TestCompile_ReportsEveryProblem(t *testing.T) {
	_, err := scopes.Compile(parse(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
    query_scopes:
      kapot: "entity.status ~="
      onbekend: "entity.geen_zo_veld == 'x'"
`))
	if err == nil {
		t.Fatal("Compile succeeded, want errors")
	}
	for _, want := range []string{"kapot", "onbekend"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing scope %q; got:\n%v", want, err)
		}
	}
	// The expression is echoed so the operator can see what failed without
	// opening the schema.
	if !strings.Contains(err.Error(), "entity.geen_zo_veld") {
		t.Errorf("error does not echo the failing expression; got:\n%v", err)
	}
}

// TestCompile_UnknownPropertyIsALoadError pins AC1: a scope over a property
// the type does not declare must fail at startup, not silently match nothing
// on every page forever.
func TestCompile_UnknownPropertyIsALoadError(t *testing.T) {
	_, err := scopes.Compile(parse(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
    query_scopes:
      actief: "entity.nietbestaand == 'x'"
`))
	if err == nil {
		t.Fatal("Compile accepted a scope over an undeclared property")
	}
}

// TestIdentity_PlainScopeNeedsNoIdentity pins the load-bearing consequence of
// compiling every scope with current_user declared: a scope that never
// mentions the identity must still evaluate on a deployment that has none.
// Without this, adding identity support would break every anonymous
// deployment's plain scopes.
func TestIdentity_PlainScopeNeedsNoIdentity(t *testing.T) {
	m := parse(t, scopedSchema)
	c, err := scopes.Compile(m)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	prog, _ := c.Lookup("taak", "actief")
	if predicatefns.RequiresCurrentUser(prog) {
		t.Fatal("a scope with no current_user reference reported as requiring one")
	}
	got, err := predicatefns.NewEvaluator(m).MatchesAs(
		context.Background(), prog, "taak", "TAAK-1", map[string]any{"status": "todo"})
	if err != nil {
		t.Fatalf("plain scope required an identity: %v", err)
	}
	if !got {
		t.Error("actief did not match a todo task")
	}
}

// TestIdentity_ScopeErrorsWithoutIdentity pins the other half: an identity
// scope with no principal is an ERROR, never a silent non-match. A non-match
// would render an empty page that looks like "you have no tasks".
func TestIdentity_ScopeErrorsWithoutIdentity(t *testing.T) {
	m := parse(t, scopedSchema)
	c, _ := scopes.Compile(m)
	prog, _ := c.Lookup("taak", "mijn")
	if !predicatefns.RequiresCurrentUser(prog) {
		t.Fatal("is_current_user scope did not report as requiring an identity")
	}
	_, err := predicatefns.NewEvaluator(m).MatchesAs(
		context.Background(), prog, "taak", "TAAK-1", map[string]any{"owner": "alice"})
	if err == nil {
		t.Error("identity scope silently non-matched without a principal; want an error")
	}
}

// TestRequiresIdentity_ReportsDefaultsOnly pins the warning surface. A NAMED
// identity scope is opt-in, so a view selecting it accepted the requirement;
// a DEFAULT one silently makes every page for that type identity-dependent.
func TestRequiresIdentity_ReportsDefaultsOnly(t *testing.T) {
	c, err := scopes.Compile(parse(t, scopedSchema))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// taak's default is a plain status check; mijn is identity but NAMED.
	if got := c.RequiresIdentity(); len(got) != 0 {
		t.Errorf("RequiresIdentity() = %v, want empty (the identity scope is named, not default)", got)
	}

	withIdentityDefault, err := scopes.Compile(parse(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {owner: {type: string}}
    query_scopes:
      default: "is_current_user(entity.owner)"
`))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := withIdentityDefault.RequiresIdentity(); len(got) != 1 || got[0] != "taak" {
		t.Errorf("RequiresIdentity() = %v, want [taak]", got)
	}
}

func TestCompile_NilAndEmpty(t *testing.T) {
	c, err := scopes.Compile(nil)
	if err != nil {
		t.Fatalf("Compile(nil): %v", err)
	}
	if _, ok := c.Lookup("taak", metamodel.AllQueryScopeName); !ok {
		t.Error("Compile(nil) did not resolve the implicit all")
	}
	if got := c.RequiresIdentity(); len(got) != 0 {
		t.Errorf("RequiresIdentity() = %v, want empty", got)
	}
}

// eachOrderSchema declares two types with several scopes, deliberately named
// so that map iteration order and sorted order differ.
const eachOrderSchema = `version: "1.0"
entities:
  taak:
    label: Taak
    plural: taken
    id_prefix: "TAAK-"
    id_type: sequential
    properties:
      status:
        type: string
    query_scopes:
      zzz: "entity.status == 'a'"
      default: "entity.status == 'b'"
  aap:
    label: Aap
    plural: apen
    id_prefix: "AAP-"
    id_type: sequential
    properties:
      status:
        type: string
    query_scopes:
      mid: "entity.status == 'c'"
relations: {}
`

// TestEach_StableOrder pins that Each yields (type, name) sorted. Its callers
// emit operator-facing diagnostics, and map iteration order would reshuffle a
// boot log on every restart, making it undiffable.
func TestEach_StableOrder(t *testing.T) {
	compiled, err := scopes.Compile(parse(t, eachOrderSchema))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var got []string
	compiled.Each(func(key scopes.Key, prog *predicate.Program) {
		if prog == nil {
			t.Errorf("Each must not yield a nil program for declared scope %v", key)
		}
		got = append(got, key.EntityType+"/"+key.Name)
	})
	want := []string{"aap/mid", "taak/default", "taak/zzz"}
	if !slices.Equal(got, want) {
		t.Errorf("Each order = %v, want %v", got, want)
	}
}

// TestEach_NilReceiver pins the documented nil contract.
func TestEach_NilReceiver(t *testing.T) {
	var c *scopes.Compiled
	c.Each(func(scopes.Key, *predicate.Program) {
		t.Fatal("nil Compiled must not yield")
	})
}

const traversalScopeSchema = `version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties: {status: {type: string}}
  feature:
    label: Feature
    id_prefix: FEAT
    properties: {status: {type: string}}
    query_scopes:
      busy: "related(entity, 'implementedBy', { status = 'in-progress' })"
relations:
  implements: {label: implements, from: [ticket], to: [feature], inverse: implementedBy}
`

// A scope using related() is validated against the schema at compile, so a
// traversal that cannot resolve is a load error naming the relation rather
// than a scope that silently matches nothing (TKT-CXQEV0).
func TestCompile_ValidatesTraversals(t *testing.T) {
	if _, err := scopes.Compile(parse(t, traversalScopeSchema)); err != nil {
		t.Fatalf("a resolvable incoming traversal must compile: %v", err)
	}
	broken := strings.Replace(traversalScopeSchema, "'implementedBy'", "'implements'", 1)
	_, err := scopes.Compile(parse(t, broken))
	if err == nil || !strings.Contains(err.Error(), `relation "implements" does not start from "feature"`) {
		t.Fatalf("expected a load error naming the relation, got %v", err)
	}
	if !strings.Contains(err.Error(), `query scope "busy"`) {
		t.Errorf("error should name the scope: %v", err)
	}
}
