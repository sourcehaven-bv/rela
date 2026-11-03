package affordances_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// relationGrantPolicy lets alice create and remove `implements` and see its
// `note` meta field, with no condition. Without an identity on the context
// none of it may apply.
const relationGrantPolicy = `
roles:
  editor:
    relations:
      ticket:
        - relation: implements
          visible:
            - field: note
assignments:
  alice: editor
`

// unboundContexts are the two shapes of a missing identity: nothing stamped,
// and a principal stamped by an edge that never resolved it.
func unboundContexts() map[string]context.Context {
	return map[string]context.Context{
		"unstamped": context.Background(),
		"stamped, unbound": principal.With(context.Background(),
			principal.Principal{User: "alice", Tool: principal.ToolDataEntry}),
	}
}

// TestRelationVerdicts_NoIdentityFailsClosed: with no resolved identity every
// relation type is neither creatable nor removable, conditional or not.
func TestRelationVerdicts_NoIdentityFailsClosed(t *testing.T) {
	t.Parallel()
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, policyFromYAML(t, relationGrantPolicy)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e := ticket("T-1", nil)

	// Control: the bound caller gets the grant, so a denial below is the
	// missing identity and not the policy.
	if v, ok := r.RelationVerdicts(ctxFor(t, r, "alice"), e).Types["implements"]; ok && !v.Creatable {
		t.Fatalf("control: bound alice cannot create implements: %+v", v)
	}

	for name, ctx := range unboundContexts() {
		for verdict, got := range map[string]affordances.RelationVerdicts{
			"RelationVerdicts":              r.RelationVerdicts(ctx, e),
			"UnconditionalRelationVerdicts": r.UnconditionalRelationVerdicts(ctx, e),
		} {
			v, ok := got.Types["implements"]
			if !ok || v.Creatable || v.Removable {
				t.Errorf("%s, %s: implements = %+v, want neither creatable nor removable", name, verdict, v)
			}
			if v.Attribution["create"] != "no-identity" {
				t.Errorf("%s, %s: attribution = %v, want no-identity", name, verdict, v.Attribution)
			}
		}
	}
}

// TestRelationFieldVerdicts_NoIdentityFailsClosed: with no resolved identity
// every meta value on the edge is hidden, including one no grant mentions.
func TestRelationFieldVerdicts_NoIdentityFailsClosed(t *testing.T) {
	t.Parallel()
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, policyFromYAML(t, relationGrantPolicy)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e := ticket("T-1", nil)
	keys := []string{"note", "free_form"}

	if got := r.RelationFieldVerdicts(ctxFor(t, r, "alice"), e, "implements", keys); got["note"] {
		t.Fatalf("control: bound alice: note hidden: %v", got)
	}
	for name, ctx := range unboundContexts() {
		got := r.RelationFieldVerdicts(ctx, e, "implements", keys)
		for _, k := range keys {
			if v, ok := got[k]; !ok || v {
				t.Errorf("%s: %s = (%v, present %v), want hidden", name, k, v, ok)
			}
		}
	}
}

// TestTransitionVerdicts_NoIdentityFailsClosed: with no resolved identity the
// transition guard holds no permission, so a guarded edge is not performable.
func TestTransitionVerdicts_NoIdentityFailsClosed(t *testing.T) {
	t.Parallel()
	r := newTransitionResolver(t, "review-it")
	e := ticket("T-1", map[string]any{"status": "open"})

	if v, _ := verdictByTo(r.TransitionVerdicts(ctxFor(t, r, "alice"), e)["status"], "review"); !v.Allowed {
		t.Fatalf("control: bound alice cannot move open→review: %+v", v)
	}
	for name, ctx := range unboundContexts() {
		v, ok := verdictByTo(r.TransitionVerdicts(ctx, e)["status"], "review")
		if !ok || v.Allowed {
			t.Errorf("%s: open→review = %+v (present %v), want present and not allowed", name, v, ok)
		}
	}
}
