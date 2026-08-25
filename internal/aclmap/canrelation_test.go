package aclmap_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
)

// spawntWorldPolicy is the outage config from TKT-XZEY plus the grant that
// makes the edge expressible. What matters for these tests is what is ABSENT:
// no `create: [terugkerend]`.
const spawntWorldPolicy = `
roles:
  scheduler-system:
    read: ["*"]
    create: [taak]
    update: [taak, terugkerend]
    permissions: [create-spawnt]
  bystander:
    read: ["*"]
assignments:
  SCHED: scheduler-system
  NOBODY: bystander
relation_grants:
  spawnt:
    create: create-spawnt
`

func spawntWorld(t *testing.T, policy string) *world {
	t.Helper()
	return buildWorld(t, policy,
		[]ent{
			{"SCHED", "person"}, {"NOBODY", "person"},
			{"TERUG-1", "terugkerend"}, {"TAAK-1", "taak"},
		},
		nil,
	)
}

// TestCanRelation_AllowViaRelationGrant is the check that would have caught
// the outage: it answers the relation-shaped question no tool could ask.
func TestCanRelation_AllowViaRelationGrant(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	res, err := w.eng.CanRelation(context.Background(), "SCHED", acl.VerbCreate, "spawnt", "TERUG-1")
	if err != nil {
		t.Fatalf("CanRelation: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("SCHED holds create-spawnt and must be allowed: %s", res.Reason)
	}
	if res.RuleKind != "relation-grant" || res.RuleID != "create-spawnt" {
		t.Errorf("rule = %q/%q, want relation-grant/create-spawnt — an operator "+
			"needs to know WHICH configuration produced the allow, since the two "+
			"allow paths need different edits to revoke", res.RuleKind, res.RuleID)
	}
	if res.FromType != "terugkerend" {
		t.Errorf("from_type = %q, want terugkerend", res.FromType)
	}
}

// TestCanRelation_ReproducesTheOutageStatically is the acceptance test for
// this command's reason to exist. The SAME policy that passed `acl map` and
// `acl audit` during the incident must now report DENY — and name the gate.
func TestCanRelation_ReproducesTheOutageStatically(t *testing.T) {
	t.Parallel()
	const outageConfig = `
roles:
  scheduler-system:
    read: ["*"]
    create: [taak]
    update: [taak, terugkerend]
assignments:
  SCHED: scheduler-system
`
	w := spawntWorld(t, outageConfig)
	res, err := w.eng.CanRelation(context.Background(), "SCHED", acl.VerbCreate, "spawnt", "TERUG-1")
	if err != nil {
		t.Fatalf("CanRelation: %v", err)
	}
	if res.Allowed {
		t.Fatal("the outage config must report DENY; reporting ALLOW here is the " +
			"exact divergence between static check and runtime that caused it")
	}
	if !strings.Contains(res.Reason, "relations from type") {
		t.Errorf("reason %q does not explain WHY; a bare no is what acl map "+
			"already gave", res.Reason)
	}
}

// TestCanRelation_DeniesWhenPermissionNotHeld pins that the grant is checked
// against the principal, not merely declared in the policy.
func TestCanRelation_DeniesWhenPermissionNotHeld(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	res, err := w.eng.CanRelation(context.Background(), "NOBODY", acl.VerbCreate, "spawnt", "TERUG-1")
	if err != nil {
		t.Fatalf("CanRelation: %v", err)
	}
	if res.Allowed {
		t.Fatal("NOBODY holds neither the permission nor the source-type grant")
	}
}

// TestCanRelation_VerbsAreIndependent pins that a create-only relation grant
// answers only for create.
//
// DELETE is the verb to assert on here: the fixture role holds
// `update: [taak, terugkerend]`, so an update on a terugkerend-sourced edge is
// legitimately allowed by the SOURCE-TYPE grant — and the result attributes it
// to role-grant, not relation-grant. Asserting "update denied" would be
// asserting the wrong thing, and is how a test ends up pinning a bug.
func TestCanRelation_VerbsAreIndependent(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	ctx := context.Background()

	res, err := w.eng.CanRelation(ctx, "SCHED", acl.VerbDelete, "spawnt", "TERUG-1")
	if err != nil {
		t.Fatalf("CanRelation(delete): %v", err)
	}
	if res.Allowed {
		t.Error("delete allowed by a create-only relation grant with no delete on " +
			"the source type")
	}

	// Update IS allowed, but via the pre-existing source-type grant — the
	// relation grant must not be credited for it.
	upd, err := w.eng.CanRelation(ctx, "SCHED", acl.VerbUpdate, "spawnt", "TERUG-1")
	if err != nil {
		t.Fatalf("CanRelation(update): %v", err)
	}
	if !upd.Allowed {
		t.Fatalf("update should be allowed by update: [terugkerend]: %s", upd.Reason)
	}
	if upd.RuleKind != "role-grant" {
		t.Errorf("update attributed to %q, want role-grant — the create-only "+
			"relation grant must not be credited for a source-type allow", upd.RuleKind)
	}
}

// TestCanRelation_RejectsReadVerb pins that read is not askable here. There is
// no relation-level read grant to report on — visibility is derived from both
// endpoints — so answering would invent a semantic.
func TestCanRelation_RejectsReadVerb(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	if _, err := w.eng.CanRelation(context.Background(), "SCHED", acl.VerbRead, "spawnt", "TERUG-1"); err == nil {
		t.Fatal("read must be rejected, not answered")
	}
	if aclmap.RelationVerbValid(acl.VerbRead) {
		t.Error("RelationVerbValid(read) = true")
	}
	for _, v := range []acl.Verb{acl.VerbCreate, acl.VerbUpdate, acl.VerbDelete} {
		if !aclmap.RelationVerbValid(v) {
			t.Errorf("RelationVerbValid(%s) = false", v)
		}
	}
}

// TestCanRelation_MissingSourceIsNotADeny pins that a typo'd entity id errors
// distinctly. A typo that reads as a considered "no" is a green attestation on
// nothing — the failure mode this command exists to remove.
func TestCanRelation_MissingSourceIsNotADeny(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	_, err := w.eng.CanRelation(context.Background(), "SCHED", acl.VerbCreate, "spawnt", "NO-SUCH")
	if err == nil {
		t.Fatal("a missing source entity must error, never report a plain deny")
	}
	if !errors.Is(err, aclmap.ErrEntityNotFound) {
		t.Errorf("error %v is not ErrEntityNotFound, so the CLI cannot special-case it", err)
	}
}
