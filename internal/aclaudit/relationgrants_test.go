package aclaudit_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclaudit"
)

// noExternalPerms reports that no permission is referenced outside acl.yaml.
// A nil PermissionConsumer would SKIP A7 entirely (see the Audit godoc), which
// would make the A7 tests below vacuous — they must run the check, not
// suppress it.
type noExternalPerms struct{}

func (noExternalPerms) UsedPermissions() []string { return nil }

func findRule(t *testing.T, findings []aclaudit.Finding, rule string) *aclaudit.Finding {
	t.Helper()
	for i := range findings {
		if findings[i].Rule == rule {
			return &findings[i]
		}
	}
	return nil
}

// TestA6b_InertRelationGrantIsReported is the operator-facing half of the
// relation_grants safety net: config that READS as a grant and grants nothing
// is the failure mode behind TKT-XZEY's outage, and the audit is where an
// operator finds it before a write does.
func TestA6b_InertRelationGrantIsReported(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  scheduler:
    read: ["*"]
relation_grants:
  spawnt:
    create: create-spawnt
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := findRule(t, aclaudit.Audit(p, nil, noExternalPerms{}), "A6b-inert-relation-grant")
	if got == nil {
		t.Fatal("no A6b finding; a relation_grants entry naming a permission no " +
			"role grants is inert, and nothing else reports it")
	}
	if got.Severity != aclaudit.Medium {
		t.Errorf("severity = %v, want medium — an inert grant fails OPEN in the "+
			"sense that an operator may have relaxed an entity grant against it",
			got.Severity)
	}
	if !strings.Contains(got.Detail, "create-spawnt") {
		t.Errorf("detail %q does not name the permission", got.Detail)
	}
}

// TestA6b_SilentWhenTheGrantIsBacked is the counterweight — a correctly-wired
// grant must produce no finding, or the check is noise.
func TestA6b_SilentWhenTheGrantIsBacked(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  scheduler:
    read: ["*"]
    permissions: [create-spawnt]
relation_grants:
  spawnt:
    create: create-spawnt
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := findRule(t, aclaudit.Audit(p, nil, noExternalPerms{}), "A6b-inert-relation-grant"); got != nil {
		t.Errorf("A6b fired on a backed grant: %s", got.Detail)
	}
}

// TestA6b_ShorthandReportedOnce pins that the shorthand — which names one
// permission for three verbs — produces one finding, not three.
func TestA6b_ShorthandReportedOnce(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  scheduler: {read: ["*"]}
relation_grants:
  spawnt:
    permission: manage-spawnt
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var n int
	for _, f := range aclaudit.Audit(p, nil, noExternalPerms{}) {
		if f.Rule == "A6b-inert-relation-grant" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("A6b findings = %d, want 1 — the shorthand names one permission", n)
	}
}

// TestA7_RelationGrantsCountAsAPermissionConsumer is a regression test for a
// false positive this feature introduced.
//
// A7 reports a permission no gate references. Before relation_grants there was
// exactly one consumer (requires_permission); now there are two. Counting only
// the first reports EVERY correctly-configured relation grant as dead — a
// false finding on precisely the config the audit is meant to make
// trustworthy, which is how operators learn to ignore an audit.
func TestA7_RelationGrantsCountAsAPermissionConsumer(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  scheduler:
    read: ["*"]
    permissions: [create-spawnt]
relation_grants:
  spawnt:
    create: create-spawnt
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := findRule(t, aclaudit.Audit(p, nil, noExternalPerms{}), "A7-dead-permission"); got != nil {
		t.Errorf("A7 called a permission dead that relation_grants consumes: %s", got.Detail)
	}
}

// TestA7_StillReportsGenuinelyDeadPermissions is the counterweight to the fix
// above: widening "used" must not blind A7 entirely.
func TestA7_StillReportsGenuinelyDeadPermissions(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  scheduler:
    read: ["*"]
    permissions: [nobody-references-me]
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := findRule(t, aclaudit.Audit(p, nil, noExternalPerms{}), "A7-dead-permission"); got == nil {
		t.Error("A7 no longer reports a genuinely unreferenced permission")
	}
}
