package appbuild

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// traversalFunc refuses rather than answers false: a false here would read
// as a legitimate "no match" and widen a negated traversal.
func TestTraversalFunc_Refuses(t *testing.T) {
	spec := predicate.TraversalSpec{Subject: "entity", Path: []string{"implementedBy"}}
	answers := traversalAnswers{spec.Key(): {"FEAT-1": true}}
	row := predicate.NewRecord(map[string]predicate.Value{"id": predicate.NewString("FEAT-1")})

	ok, err := answers.traversalFunc("FEAT-1")(row, spec)
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
			if _, err := answers.traversalFunc("FEAT-1")(tc.subject, tc.spec); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
