package predicatefns

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// traversalMeta mirrors the shape that makes unions hard in the live tickets
// schema: `caused-by` points at four types, and `status` is declared on each
// with a DIFFERENT enum type — which is why a bare property reference through
// it cannot be typed.
func traversalMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "ticket_status"},
				"tags":   {Type: metamodel.PropertyTypeString, List: true},
			}},
			"concept": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: "concept_status"},
			}},
			"person": {Properties: map[string]metamodel.PropertyDef{
				"name": {Type: metamodel.PropertyTypeString},
			}},
		},
		Relations: map[string]metamodel.RelationDef{
			// Single target: no ascription needed.
			"owned-by": {From: []string{"ticket", "concept"}, To: []string{"person"}},
			// Union target: ascription required.
			"caused-by": {From: []string{"ticket"}, To: []string{"concept", "ticket"}},
		},
	}
}

func compileFor(t *testing.T, src string) *predicate.Program {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{"status": predicate.StringType}); err != nil {
		t.Fatal(err)
	}
	prog, err := predicate.Compile(env, src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return prog
}

func TestValidateTraversals(t *testing.T) {
	meta := traversalMeta()
	meta.Relations["about"] = metamodel.RelationDef{From: []string{"concept"}, To: []string{"person"}}
	for _, tc := range []struct {
		name, src, wantErr string
	}{
		{
			name: "single-target relation needs no ascription",
			src:  `related(entity, 'owned-by', { name = 'alice' })`,
		},
		{
			name: "union target with a valid ascription",
			src:  `related(entity, 'caused-by', { type = 'concept', status = 'open' })`,
		},
		{
			name:    "union target without an ascription is refused",
			src:     `related(entity, 'caused-by', { status = 'open' })`,
			wantErr: "add type=",
		},
		{
			name:    "ascription naming a non-target is refused",
			src:     `related(entity, 'caused-by', { type = 'person' })`,
			wantErr: "is not a target of",
		},
		{
			name:    "unknown relation type",
			src:     `related(entity, 'no-such-rel')`,
			wantErr: "unknown relation type",
		},
		{
			// `about` starts from concept, so traversing it FROM a ticket can
			// never match. Refusing at load names the relation; letting it
			// through would return an empty result that reads as "no data".
			name:    "relation that does not start from this type",
			src:     `related(entity, 'about')`,
			wantErr: "does not start from",
		},
		{
			name:    "undeclared property on the target",
			src:     `related(entity, 'caused-by', { type = 'concept', nope = 'x' })`,
			wantErr: `has no property "nope"`,
		},
		{
			name:    "list property is refused",
			src:     `related(entity, 'caused-by', { type = 'ticket', tags = 'x' })`,
			wantErr: "is a list",
		},
		{
			name:    "intermediate union hop is refused",
			src:     `related(entity, { 'caused-by', 'owned-by' }, { type = 'person' })`,
			wantErr: "cannot be resolved",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTraversals(meta, "ticket", compileFor(t, tc.src))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// A chain through single-target relations resolves hop by hop, so the property
// is checked against the FINAL type rather than the starting one.
func TestValidateTraversals_ChainResolvesToTheFinalType(t *testing.T) {
	meta := traversalMeta()
	meta.Relations["about"] = metamodel.RelationDef{From: []string{"concept"}, To: []string{"person"}}

	// ticket -caused-by(concept)-> concept -about-> person, filtering person.name
	if err := ValidateTraversals(meta, "ticket",
		compileFor(t, `related(entity, { 'caused-by', 'about' }, { type = 'person', name = 'alice' })`)); err == nil {
		t.Fatal("expected refusal: the first hop is a union and cannot be resolved mid-chain")
	}

	// A fully single-target chain resolves.
	meta.Relations["derives"] = metamodel.RelationDef{From: []string{"ticket"}, To: []string{"concept"}}
	if err := ValidateTraversals(meta, "ticket",
		compileFor(t, `related(entity, { 'derives', 'about' }, { name = 'alice' })`)); err != nil {
		t.Fatalf("single-target chain must resolve: %v", err)
	}
	// ...and the property is checked on the final type, not the first hop's.
	err := ValidateTraversals(meta, "ticket",
		compileFor(t, `related(entity, { 'derives', 'about' }, { status = 'open' })`))
	if err == nil || !strings.Contains(err.Error(), `has no property "status"`) {
		t.Fatalf("expected the property to be checked on person, got %v", err)
	}
}

// A nil metamodel or program must not panic: callers compile before they have
// a schema in some paths.
func TestValidateTraversals_NilInputs(t *testing.T) {
	if err := ValidateTraversals(nil, "ticket", nil); err != nil {
		t.Fatalf("nil inputs must be a no-op, got %v", err)
	}
}
