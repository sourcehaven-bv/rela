package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// stubACL is an acl.ACL implementation the switch has never been taught
// about — it stands in for a future one added without a matching arm. It
// deliberately AUTHORIZES every write, so a switch that fell through to some
// "ask the ACL" default would grant: the test proves the default arm denies
// on identity alone, not because this stub happens to say no.
type stubACL struct{}

func (stubACL) AuthorizeWrite(context.Context, acl.WriteRequest) acl.Decision {
	return acl.Decision{Allow: true}
}

// elevatedDoc is a document declaring allow_acl_bypass: read with the
// permission validateDocuments requires alongside it.
func elevatedDoc() dataentryconfig.DocumentConfig {
	return dataentryconfig.DocumentConfig{
		Script:         "docs/benchmark.lua",
		AllowACLBypass: metamodel.ACLBypassRead,
		Permission:     "report:sales",
	}
}

// TestElevatedDocument_ClosedSwitch is the table that pins the gate across
// every ACL implementation, including both the value and pointer forms of the
// nop/read-only types.
//
// The default arm is the point: an implementation nobody taught the switch
// about must not silently grant an ungated raw read.
func TestElevatedDocument_ClosedSwitch(t *testing.T) {
	t.Parallel()

	holds := permGate{perms: map[string]bool{"report:sales": true}}

	for _, tc := range []struct {
		name    string
		aclImpl acl.ACL
		want    bool
	}{
		{name: "nil ACL denies (wiring bug fails closed)", aclImpl: nil},
		{name: "NopACL value denies", aclImpl: acl.NopACL{}},
		{name: "NopACL pointer denies", aclImpl: &acl.NopACL{}},
		{name: "ReadOnlyACL value denies", aclImpl: acl.ReadOnlyACL{}},
		{name: "ReadOnlyACL pointer denies", aclImpl: &acl.ReadOnlyACL{}},
		{name: "nil *Declarative denies", aclImpl: (*acl.Declarative)(nil)},
		{name: "unknown implementation denies (default arm)", aclImpl: stubACL{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := withReadGate(t.Context(), holds)
			if got := authorizeElevatedDocument(ctx, tc.aclImpl, elevatedDoc()); got != tc.want {
				t.Errorf("authorizeElevatedDocument = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestElevatedDocument_NopACLDeniesDespitePermittingGate is the RR-CWWJGW
// canary, in the shape TestNavPermission_ReadOnlyArmIsExplicit established.
//
// Under NopACL and ReadOnlyACL readGateFromContext returns nopReadGate, whose
// HoldsPermission returns TRUE unconditionally. A gate written against the read
// gate alone would therefore ALLOW an elevated render on a policy-less
// deployment — serving whatever the script reads to every caller.
//
// Attaching a gate that GRANTS the permission proves the denial comes from the
// explicit arm rather than from the gate happening to agree: if someone deletes
// the NopACL arm, the value falls to the default arm (which also denies) — so
// to make this test meaningful the assertion is that the answer is deny while a
// permitting gate is present, i.e. the gate was never consulted.
func TestElevatedDocument_NopACLDeniesDespitePermittingGate(t *testing.T) {
	t.Parallel()

	// A gate that grants everything — the nopReadGate posture, made explicit.
	ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"report:sales": true}})

	for _, impl := range []acl.ACL{acl.NopACL{}, &acl.NopACL{}, acl.ReadOnlyACL{}, &acl.ReadOnlyACL{}} {
		if authorizeElevatedDocument(ctx, impl, elevatedDoc()) {
			t.Errorf("%T allowed an elevated render with no configured policy; the permission "+
				"names a capability nothing can withhold there, so the document would serve "+
				"its raw reads to every principal", impl)
		}
	}
}

// TestElevatedDocument_UnelevatedIsUnaffected pins that this gate is inert for
// the overwhelmingly common document. The nested CLAUDE.md is explicit that
// `permission:` must not be hardened into a required field for ordinary
// documents; this ticket narrows that only for elevated ones.
func TestElevatedDocument_UnelevatedIsUnaffected(t *testing.T) {
	t.Parallel()

	plain := dataentryconfig.DocumentConfig{Script: "docs/report.lua"}
	gated := dataentryconfig.DocumentConfig{Script: "docs/report.lua", Permission: "report:sales"}
	ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{}})

	// Every implementation, including the ones that deny elevation outright.
	for _, impl := range []acl.ACL{nil, acl.NopACL{}, acl.ReadOnlyACL{}, stubACL{}} {
		if !authorizeElevatedDocument(ctx, impl, plain) {
			t.Errorf("%T: an unelevated document must pass this gate untouched", impl)
		}
		// A permission on an unelevated document is still enforced, but by
		// gateDocumentPermission — not here.
		if !authorizeElevatedDocument(ctx, impl, gated) {
			t.Errorf("%T: an unelevated document with a permission must pass THIS gate; "+
				"gateDocumentPermission is what enforces it", impl)
		}
	}
}

// TestElevatedDocument_DeclarativeRequiresHeldPermission pins the arm that
// actually authorizes: under a real policy the principal must hold it.
func TestElevatedDocument_DeclarativeRequiresHeldPermission(t *testing.T) {
	t.Parallel()

	policy := &acl.Declarative{}

	t.Run("holder allowed", func(t *testing.T) {
		t.Parallel()
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"report:sales": true}})
		if !authorizeElevatedDocument(ctx, policy, elevatedDoc()) {
			t.Error("a principal holding the permission must render the elevated document")
		}
	})

	t.Run("non-holder denied", func(t *testing.T) {
		t.Parallel()
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"other:perm": true}})
		if authorizeElevatedDocument(ctx, policy, elevatedDoc()) {
			t.Error("a principal without the permission must not render the elevated document")
		}
	})

	t.Run("empty permission denied even under a permitting gate", func(t *testing.T) {
		t.Parallel()
		// validateDocuments rejects this config, so it is unreachable today.
		// The guard must not depend on validation having run: a new
		// construction path that skipped it would otherwise fail open.
		doc := elevatedDoc()
		doc.Permission = ""
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"report:sales": true}})
		if authorizeElevatedDocument(ctx, policy, doc) {
			t.Error("an elevated document with no permission must be denied, not allowed " +
				"on the assumption that config validation caught it")
		}
	})
}
