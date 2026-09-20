package queryplan

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func traversalIndexMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		// The enum types must be DECLARED for StringShaped to accept them —
		// an undeclared custom type is not pushdown-eligible, which is the
		// same gate the query-side pushdown applies.
		Types: map[string]metamodel.CustomType{
			"ticket_status":  {Values: []string{"open", "done"}},
			"concept_status": {Values: []string{"open", "done"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"status": {Type: "ticket_status"}}},
			"concept": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "concept_status"},
				"count":  {Type: metamodel.PropertyTypeInteger},
				"tags":   {Type: metamodel.PropertyTypeString, List: true},
			}},
			"person": {Properties: map[string]metamodel.PropertyDef{"name": {Type: metamodel.PropertyTypeString}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"derives":   {From: []string{"ticket"}, To: []string{"concept"}},
			"about":     {From: []string{"concept"}, To: []string{"person"}},
			"caused-by": {From: []string{"ticket"}, To: []string{"concept", "ticket"}},
		},
	}
}

func progFor(t *testing.T, src string) *predicate.Program {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{"status": predicate.StringType}); err != nil {
		t.Fatal(err)
	}
	p, err := predicate.Compile(env, src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return p
}

// The whole point: the index lands on the TRAVERSED-TO type, not the queried
// one. DerivedObjectSpec carries a single Type and the DDL is partial on it,
// so no number of extra properties on the query's own spec can produce this.
func TestTraversalIndexSpecs_IndexesTheTargetType(t *testing.T) {
	got := TraversalIndexSpecs(
		progFor(t, `related(entity, 'derives', { status = 'open' })`),
		traversalIndexMeta(), "ticket")

	if len(got) != 1 {
		t.Fatalf("expected 1 spec, got %d: %+v", len(got), got)
	}
	if got[0].Type != "concept" {
		t.Errorf("index must be on the traversed-to type, got %q", got[0].Type)
	}
	if got[0].Kind != store.DerivedQueryIndex {
		t.Errorf("kind = %v", got[0].Kind)
	}
	if len(got[0].Properties) != 1 || got[0].Properties[0] != "status" {
		t.Errorf("properties = %v", got[0].Properties)
	}
}

// A chain indexes the FINAL hop's type: intermediate hops are resolved by
// relation-type lookups, which the relations indexes already serve.
func TestTraversalIndexSpecs_ChainIndexesTheFinalType(t *testing.T) {
	got := TraversalIndexSpecs(
		progFor(t, `related(entity, { 'derives', 'about' }, { name = 'alice' })`),
		traversalIndexMeta(), "ticket")
	if len(got) != 1 || got[0].Type != "person" {
		t.Fatalf("expected one spec on person, got %+v", got)
	}
}

func TestTraversalIndexSpecs_DerivesNothingWhenUnresolvable(t *testing.T) {
	meta := traversalIndexMeta()
	for _, tc := range []struct{ name, src string }{
		// An unresolved union: ValidateTraversals refuses these at load, but
		// deriving nothing is the safe answer if one reaches here.
		{"union without ascription", `related(entity, 'caused-by', { status = 'open' })`},
		{"unknown relation", `related(entity, 'no-such', { status = 'open' })`},
		// Not string-shaped: byte order is not numeric order, so the store
		// comparison would disagree with the Go pass.
		{"integer property", `related(entity, 'derives', { count = '3' })`},
		{"list property", `related(entity, 'derives', { tags = 'x' })`},
		// No constrained properties at all: the hop is pure edge existence.
		{"no properties", `related(entity, 'derives')`},
		{"undeclared property", `related(entity, 'derives', { nope = 'x' })`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := TraversalIndexSpecs(progFor(t, tc.src), meta, "ticket"); len(got) != 0 {
				t.Fatalf("expected no specs, got %+v", got)
			}
		})
	}
}

// Two traversals onto the same type share ONE spec, and the property set is
// sorted and deduplicated — the reconciler treats an absent desired object as
// permission to DROP, so an unstable set would churn indexes between runs.
func TestTraversalIndexSpecs_MergesAndIsDeterministic(t *testing.T) {
	src := `related(entity, 'derives', { status = 'open' }) or ` +
		`related(entity, 'caused-by', { type = 'concept', status = 'done' })`
	got := TraversalIndexSpecs(progFor(t, src), traversalIndexMeta(), "ticket")
	if len(got) != 1 {
		t.Fatalf("both traversals target concept; expected 1 merged spec, got %+v", got)
	}
	if len(got[0].Properties) != 1 || got[0].Properties[0] != "status" {
		t.Fatalf("duplicate property must be compacted, got %v", got[0].Properties)
	}
}

func TestTraversalIndexSpecs_NilInputs(t *testing.T) {
	if got := TraversalIndexSpecs(nil, traversalIndexMeta(), "ticket"); got != nil {
		t.Errorf("nil program must derive nothing, got %+v", got)
	}
	if got := TraversalIndexSpecs(progFor(t, `related(entity, 'derives', {status='x'})`), nil, "ticket"); got != nil {
		t.Errorf("nil metamodel must derive nothing, got %+v", got)
	}
}

// Index derivation and validation must resolve a chain by the SAME rules.
// Before they shared ResolveTraversalTarget they did not: derivation ignored
// the `from:` side and treated a zero-target relation as merely unresolvable.
// The drift direction is the dangerous one — a condition that loads fine whose
// index is silently never derived, so the query scans every row of the target
// type. These are the cases where the two used to disagree.
func TestTraversalIndexSpecs_AgreesWithValidation(t *testing.T) {
	meta := traversalIndexMeta()
	// `about` starts from concept, not ticket: validation refuses it, so
	// derivation must not resolve it either.
	meta.Relations["about"] = metamodel.RelationDef{From: []string{"concept"}, To: []string{"person"}}
	// A relation declaring no target at all.
	meta.Relations["dangling"] = metamodel.RelationDef{From: []string{"ticket"}}

	for _, tc := range []struct{ name, src string }{
		{"wrong from-side", `related(entity, 'about', { name = 'alice' })`},
		{"no declared target", `related(entity, 'dangling', { status = 'open' })`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prog := progFor(t, tc.src)
			if err := predicatefns.ValidateTraversals(meta, "ticket", prog); err == nil {
				t.Fatal("validation should refuse this shape")
			}
			if got := TraversalIndexSpecs(prog, meta, "ticket"); len(got) != 0 {
				t.Fatalf("derivation must agree with validation and derive nothing, got %+v", got)
			}
		})
	}
}
