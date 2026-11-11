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

// RelationMetaRestricted is per caller and ignores which keys the edge has.
func TestRelationMetaRestricted(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    read: [ticket]
    relations:
      ticket:
        - relation: has-planning
          visible:
            - field: note
  reader:
    read: [ticket]
assignments:
  alice: triager
  bob: reader
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e := ticket("T-1", nil)
	for _, tc := range []struct {
		user, relType string
		want          bool
	}{
		{"alice", "has-planning", true}, // a visible: list hides every other key
		{"alice", "implements", false},  // no list for this type
		{"bob", "has-planning", false},  // no list at all
	} {
		if got := affordances.RelationMetaRestricted(ctxFor(t, r, tc.user), r, e, tc.relType); got != tc.want {
			t.Errorf("%s %s: got %v, want %v", tc.user, tc.relType, got, tc.want)
		}
	}
	for name, ctx := range unboundContexts() {
		if !affordances.RelationMetaRestricted(ctx, r, e, "implements") {
			t.Errorf("%s: no identity must be restricted", name)
		}
	}
	if affordances.RelationMetaRestricted(ctxFor(t, r, "alice"), r, nil, "has-planning") {
		t.Error("nil source: want false")
	}
}
