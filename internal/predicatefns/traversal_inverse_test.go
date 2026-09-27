package predicatefns

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// inverseMeta is parsed rather than built as a literal: the inverse-ID index
// ([metamodel.Metamodel.InverseOwner]) is populated only by the loader.
func inverseMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      status: {type: string}
      tags: {type: string, list: true}
  bug:
    label: Bug
    id_prefix: BUG
    properties:
      status: {type: string}
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      status: {type: string}
  concept:
    label: Concept
    id_prefix: CON
    properties:
      status: {type: string}
relations:
  implements:
    label: implements
    from: [ticket]
    to: [feature]
    inverse: implementedBy
  fixes:
    label: fixes
    from: [ticket, bug]
    to: [feature]
    inverse: fixedBy
  requires:
    label: requires
    from: [feature]
    to: [concept]
    inverse: requiredBy
  related-to:
    label: related to
    from: [ticket]
    to: [ticket]
    symmetric: true
    inverse: related-to
`))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return m
}

func TestResolveTraversal_Direction(t *testing.T) {
	meta := inverseMeta(t)
	for _, tc := range []struct {
		name, from, src string
		want            []ResolvedHop
		wantErr         string
	}{
		{
			name: "inverse ID walks the edge backwards",
			from: "feature",
			src:  `related(entity, 'implementedBy', { status = 'open' })`,
			want: []ResolvedHop{{Relation: "implements", Incoming: true, Target: "ticket"}},
		},
		{
			name: "canonical name still walks forwards",
			from: "ticket",
			src:  `related(entity, 'implements')`,
			want: []ResolvedHop{{Relation: "implements", Target: "feature"}},
		},
		{
			name: "chain mixing directions",
			from: "concept",
			src:  `related(entity, { 'requiredBy', 'implementedBy' }, { status = 'open' })`,
			want: []ResolvedHop{
				{Relation: "requires", Incoming: true, Target: "feature"},
				{Relation: "implements", Incoming: true, Target: "ticket"},
			},
		},
		{
			name: "chain back out again",
			from: "ticket",
			src:  `related(entity, { 'implements', 'requires' })`,
			want: []ResolvedHop{
				{Relation: "implements", Target: "feature"},
				{Relation: "requires", Target: "concept"},
			},
		},
		{
			name:    "union on the FROM side needs an ascription",
			from:    "feature",
			src:     `related(entity, 'fixedBy', { status = 'open' })`,
			wantErr: "add type=",
		},
		{
			name: "union on the FROM side with an ascription",
			from: "feature",
			src:  `related(entity, 'fixedBy', { type = 'bug', status = 'open' })`,
			want: []ResolvedHop{{Relation: "fixes", Incoming: true, Target: "bug"}},
		},
		{
			name:    "inverse used from the FROM side is refused",
			from:    "ticket",
			src:     `related(entity, 'implementedBy')`,
			wantErr: `"implementedBy" walks "implements" backwards, which does not end at "ticket"`,
		},
		{
			name:    "canonical name used from the TO side is refused",
			from:    "feature",
			src:     `related(entity, 'implements')`,
			wantErr: `relation "implements" does not start from "feature"`,
		},
		{
			name:    "symmetric relation is refused",
			from:    "ticket",
			src:     `related(entity, 'related-to')`,
			wantErr: "is symmetric",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prog := compileFor(t, tc.src)
			hops, err := ResolveTraversal(meta, tc.from, prog.Traversals()[0])
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one mentioning %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(hops) != len(tc.want) {
				t.Fatalf("hops = %+v, want %+v", hops, tc.want)
			}
			for i := range hops {
				if hops[i] != tc.want[i] {
					t.Errorf("hop %d = %+v, want %+v", i, hops[i], tc.want[i])
				}
			}
		})
	}
}

func TestValidateTraversals_RefusesAnotherSubject(t *testing.T) {
	env := predicate.NewEnv()
	for _, v := range []string{"entity", "current_user"} {
		if err := env.DeclareVar(v, predicate.RecordType{"status": predicate.StringType}); err != nil {
			t.Fatal(err)
		}
	}
	prog, err := predicate.Compile(env, `related(current_user, 'implementedBy')`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	err = ValidateTraversals(inverseMeta(t), "feature", prog)
	if err == nil || !strings.Contains(err.Error(), `first argument must be "entity"`) {
		t.Fatalf("expected the current_user subject to be refused, got %v", err)
	}
}

// An incoming hop lands on the relation's FROM side; a relation declaring no
// FROM types leaves nothing to land on.
func TestResolveTraversal_IncomingOverEmptyFromIsRefused(t *testing.T) {
	meta := inverseMeta(t)
	def := meta.Relations["implements"]
	def.From = nil
	meta.Relations["implements"] = def
	_, err := ResolveTraversal(meta, "feature",
		compileFor(t, `related(entity, 'implementedBy')`).Traversals()[0])
	if err == nil || !strings.Contains(err.Error(), "declares no type on the side it walks to") {
		t.Fatalf("expected refusal, got %v", err)
	}
}

// Validation checks properties on the type an INCOMING hop lands on, which is
// the relation's FROM side.
func TestValidateTraversals_InverseChecksPropsOnTheFromType(t *testing.T) {
	meta := inverseMeta(t)
	err := ValidateTraversals(meta, "feature", compileFor(t, `related(entity, 'implementedBy', { tags = 'x' })`))
	if err == nil || !strings.Contains(err.Error(), `"tags" on "ticket" is a list`) {
		t.Fatalf("expected the list property on ticket to be refused, got %v", err)
	}
}
