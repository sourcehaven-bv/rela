package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

func TestRelationMetaGated(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
        - relation: implements
          create: true
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for relType, want := range map[string]bool{
		"has-planning": true,  // a visible: list
		"implements":   false, // a grant without one
		"unknown":      false,
	} {
		if got := affordances.RelationMetaGated(r, relType); got != want {
			t.Errorf("RelationMetaGated(%q) = %v, want %v", relType, got, want)
		}
	}
}
