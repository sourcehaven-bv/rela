package appbuild

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestCopyWiringIsNotInert pins the OTHER direction: the wired guard must be
// capable of REFUSING, not merely present.
//
// Without this, TestCopyDepsAreWired alone would be satisfied by a guard that
// always returns true — a different way for the wiring to be wrong, and the
// more dangerous one, because it looks configured.
//
// It asserts at the ADAPTER level rather than end-to-end: the fail-closed
// branch fires when a policy is active but no acl.Request is on the context,
// which is precisely the "served path lost its middleware" case the adapter
// exists to refuse. Driving that through a full Declarative policy would test
// the policy engine, which has its own tests; what is unproven here is the
// ADAPTER's tiering.
func TestCopyWiringIsNotInert(t *testing.T) {
	t.Parallel()

	// Policy active, no Request on ctx -> must deny / report absent.
	gate := copyReadGate{policyActive: true}
	if ok, err := gate.PermitsReadFace(context.Background(), "page", "PAGE-1", ""); ok || err != nil {
		t.Errorf("with a policy active and no acl.Request on the context, the "+
			"read gate must FAIL CLOSED: a served path that lost its "+
			"Request-attach middleware must not silently open a governed "+
			"read; got ok=%v err=%v", ok, err)
	}

	vis := copyVisibility{st: memstore.New(), redact: visibility.NopRedactor{}, policyActive: true}
	if _, found, err := vis.Get(context.Background(), "page", "PAGE-1", ""); found || err != nil {
		t.Errorf("same for the visibility gate: absent rather than ungated, "+
			"indistinguishable from a genuine miss; got found=%v err=%v", found, err)
	}

	// No policy at all -> inert, permits. Every other read on such a
	// deployment is raw too, so gating this one would be theater.
	inert := copyReadGate{policyActive: false}
	if ok, err := inert.PermitsReadFace(context.Background(), "page", "PAGE-1", ""); !ok || err != nil {
		t.Errorf("with no policy configured the gate is inert; got ok=%v err=%v", ok, err)
	}
}
