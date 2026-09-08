package appbuild_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestCompileTransitions_AlwaysSuppliesCopyGates pins the invariant that makes
// the #1437 guard hold at the WIRING layer rather than only at the constructor.
//
// entitymanager.New refuses a policy-backed Deps whose copy read gates are
// nil. That is the backstop. This is the reason the backstop should never
// fire in production: every wiring site — assemble and the appbuildtest
// fixture alike — takes both gates from the TransitionWiring bundle, and the
// bundle always populates them.
//
// Before this, the gates were built inline in buildEntityManager and
// hand-copied into the fixture, which left the fixture passing a policy-backed
// ACL with both gates nil. Asserting non-nil here is the cheap check that the
// single source stayed single; if someone reintroduces an inline build at one
// site, the other site's gates go nil and this fails.
//
// The posture the gates take (inert without a policy, fail-closed with one) is
// tested where it is observable — through actual copy behavior in
// copywiring_test.go and entitymanager's own suite. This test asserts only
// that a gate EXISTS, because a nil is the one failure that is invisible at
// the call site.
func TestCompileTransitions_AlwaysSuppliesCopyGates(t *testing.T) {
	t.Parallel()

	meta, err := metamodel.Parse([]byte(copyWiringMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}

	declarative := func(t *testing.T) acl.ACL {
		t.Helper()
		policy := &acl.Policy{
			Roles:       map[string]acl.RoleDef{"author": {Read: []string{"page"}}},
			Assignments: map[string]string{"alice": "author"},
		}
		if verr := policy.Validate(); verr != nil {
			t.Fatalf("policy must load: %v", verr)
		}
		d, derr := acl.NewDeclarative(policy, acl.NullGraph{}, acl.NullGraphQueryer{})
		if derr != nil {
			t.Fatalf("NewDeclarative: %v", derr)
		}
		return d
	}

	// Both tiers, because the no-policy one is where a nil would be easiest to
	// justify — and it is exactly the tier the fixture used to sit in while
	// callers could hand it a real policy.
	for _, tc := range []struct {
		name    string
		aclFunc func(*testing.T) acl.ACL
	}{
		{"no policy", func(*testing.T) acl.ACL { return acl.NopACL{} }},
		{"compiled policy", declarative},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tw, cerr := appbuild.CompileTransitions(meta, memstore.New(), tc.aclFunc(t))
			if cerr != nil {
				t.Fatalf("CompileTransitions: %v", cerr)
			}
			if tw.ReadGate == nil {
				t.Error("ReadGate must never be nil: a nil copy read gate makes the " +
					"copy path read its source ungated, and every wiring site now " +
					"takes this field verbatim")
			}
			if tw.Visibility == nil {
				t.Error("Visibility must never be nil: a nil field gate lets a " +
					"cross-entity copy launder `visible:`-hidden properties into an " +
					"entity with a different audience")
			}
		})
	}
}
