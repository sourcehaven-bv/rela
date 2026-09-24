package relresolve

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// For refuses rather than answers false: a false here would read
// as a legitimate "no match" and widen a negated traversal.
func TestAnswersFor_Refuses(t *testing.T) {
	spec := predicate.TraversalSpec{Subject: "entity", Path: []string{"implementedBy"}}
	answers := Answers{asked: map[string]bool{"FEAT-1": true}, bySpec: map[string]map[string]bool{spec.Key(): {"FEAT-1": true}}}
	row := predicate.NewRecord(map[string]predicate.Value{"id": predicate.NewString("FEAT-1")})

	ok, err := answers.For("FEAT-1")(row, spec)
	if err != nil || !ok {
		t.Fatalf("answered traversal: got (%v, %v), want (true, nil)", ok, err)
	}

	cases := []struct {
		name    string
		subject predicate.Value
		spec    predicate.TraversalSpec
	}{
		{"subject is not a record", predicate.NewString("FEAT-1"), spec},
		{"subject is another row", predicate.NewRecord(map[string]predicate.Value{
			"id": predicate.NewString("FEAT-2"),
		}), spec},
		{"traversal was not answered", row, predicate.TraversalSpec{Subject: "entity", Path: []string{"other"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := answers.For("FEAT-1")(tc.subject, tc.spec); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// A row outside the answered set is refused, not read as "no match": a shared
// ctx may evaluate rows (includes, neighbours) the answers never covered.
func TestAnswersFor_RefusesUnaskedRow(t *testing.T) {
	spec := predicate.TraversalSpec{Subject: "entity", Path: []string{"implementedBy"}}
	answers := Answers{asked: map[string]bool{"FEAT-1": true}, bySpec: map[string]map[string]bool{spec.Key(): {}}}
	row := predicate.NewRecord(map[string]predicate.Value{"id": predicate.NewString("FEAT-2")})
	if _, err := answers.For("FEAT-2")(row, spec); err == nil {
		t.Fatal("want an error for a row the answers were not computed for")
	}
}
