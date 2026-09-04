package acl_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// worldPolicy builds a policy that grants read on a non-default world, so
// the refusal's trigger half is satisfied and each case below varies only
// the escalation half.
func worldPolicy(t *testing.T, roles map[string]acl.RoleDef,
	assignments map[string]string, rels map[string]acl.RoleRelationDef,
) *acl.Policy {
	t.Helper()
	return &acl.Policy{Roles: roles, Assignments: assignments, RoleRelations: rels}
}

// TestRefusal_ReadOnlyRoleHoldingWorldGrantCountsAsEscalation is the
// load-bearing case, and the reason the refusal does not key on
// [acl.RoleDef.IsPrivileged].
//
// IsPrivileged excludes Read by design (RR-LXI3NW): "a read-everything role
// is a visibility choice, not an escalation path". True for the default
// world. FALSE once a read grant can name a non-default world — at which
// point self-granting a read-only role IS the leak the refusal exists to
// prevent.
//
// Both arms of the refusal as originally specified return false here: A1
// because membership is gated, and the A2 predicate because `viewer` is not
// IsPrivileged. Without the world term, this policy loads clean and one
// `owns` edge write buys a published-world read.
func TestRefusal_ReadOnlyRoleHoldingWorldGrantCountsAsEscalation(t *testing.T) {
	t.Parallel()
	p := worldPolicy(t,
		map[string]acl.RoleDef{
			"viewer": {Read: []string{"world:published"}},
			"admin":  {Permissions: []string{"delegate-admin"}},
		},
		map[string]string{"admins": "admin"},
		map[string]acl.RoleRelationDef{
			"member-of": {RequiresPermission: "delegate-admin"}, // A1: gated
			"owns":      {Confers: "viewer"},                    // ungated
		},
	)
	if err := p.Validate(); err == nil {
		t.Fatal("policy loaded, but one `owns` edge write self-grants read on " +
			"world:published — the refusal must fire on a role that is " +
			"escalation-relevant only by its world grant")
	} else if !strings.Contains(err.Error(), "owns") {
		t.Errorf("refusal must name the offending relation; got: %v", err)
	}

	// Control: the SAME policy without the world grant keeps booting. That
	// is the backward-compatibility promise — a read-only role conferred by
	// an ungated relation is not, by itself, something to refuse.
	if !p.Roles["viewer"].IsPrivileged() {
		q := worldPolicy(t,
			map[string]acl.RoleDef{
				"viewer": {Read: []string{"page"}},
				"admin":  {Permissions: []string{"delegate-admin"}},
			},
			map[string]string{"admins": "admin"},
			map[string]acl.RoleRelationDef{
				"member-of": {RequiresPermission: "delegate-admin"},
				"owns":      {Confers: "viewer"},
			},
		)
		if err := q.Validate(); err != nil {
			t.Errorf("a policy with no non-default world grant must still load: %v", err)
		}
	}
}

func TestWorldGrantRefusal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		policy  *acl.Policy
		refuse  bool
		mustSay string
	}{
		{
			name: "A1: ungated membership + privileged assignment + world grant",
			policy: worldPolicy(t,
				map[string]acl.RoleDef{
					"admin":    {Update: []string{"page"}, Read: []string{"page"}},
					"everyone": {Read: []string{"world:published"}},
				},
				map[string]string{"admins": "admin"},
				nil, // member-of ungated
			),
			refuse:  true,
			mustSay: "member-of",
		},
		{
			name: "A2: ungated role-relation confers a privileged role",
			policy: worldPolicy(t,
				map[string]acl.RoleDef{
					"editor":   {Update: []string{"page"}, Read: []string{"page"}},
					"everyone": {Read: []string{"world:published"}},
				},
				nil,
				map[string]acl.RoleRelationDef{"owns": {Confers: "editor"}},
			),
			refuse:  true,
			mustSay: "owns",
		},
		{
			name: "gated everywhere: loads",
			policy: worldPolicy(t,
				map[string]acl.RoleDef{
					"editor":   {Update: []string{"page"}, Read: []string{"page"}},
					"everyone": {Read: []string{"world:published"}},
					"admin":    {Permissions: []string{"delegate-admin"}},
				},
				map[string]string{"admins": "admin"},
				map[string]acl.RoleRelationDef{
					"member-of": {RequiresPermission: "delegate-admin"},
					"owns":      {Confers: "editor", RequiresPermission: "delegate-owns"},
				},
			),
			refuse: false,
		},
		{
			name: "NO world grant: ungated membership still BOOTS (warn-only)",
			policy: worldPolicy(t,
				map[string]acl.RoleDef{"admin": {Update: []string{"page"}, Read: []string{"page"}}},
				map[string]string{"admins": "admin"},
				nil,
			),
			refuse: false,
		},
		{
			name: "world:default only is not a non-default world grant",
			policy: worldPolicy(t,
				map[string]acl.RoleDef{
					"admin":    {Update: []string{"page"}, Read: []string{"page"}},
					"everyone": {Read: []string{"world:default", "page"}},
				},
				map[string]string{"admins": "admin"},
				nil,
			),
			refuse: false,
		},
		{
			name: "read-only assigned role, world grant, ungated membership: " +
				"REFUSED because the assigned role holds the world",
			policy: worldPolicy(t,
				map[string]acl.RoleDef{
					"reader": {Read: []string{"page", "world:published"}},
				},
				map[string]string{"readers": "reader"},
				nil,
			),
			refuse:  true,
			mustSay: "member-of",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.policy.Validate()
			if tc.refuse && err == nil {
				t.Fatal("expected a load refusal, got none")
			}
			if !tc.refuse && err != nil {
				t.Fatalf("expected the policy to load, got: %v", err)
			}
			if tc.mustSay != "" && !strings.Contains(err.Error(), tc.mustSay) {
				t.Errorf("refusal must name %q; got: %v", tc.mustSay, err)
			}
			if tc.refuse {
				// The error must be actionable: name the fix and the docs.
				for _, want := range []string{"requires_permission", "docs/acl-security.md", "rela acl audit"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal must mention %q; got: %v", want, err)
					}
				}
			}
		})
	}
}

// TestRefusal_OverRefusesUnreachableChain_Deliberately pins the accepted
// cost of the necessary-condition design (Ruling 8).
//
// `owns` confers a privileged role but that role holds NO permission
// gating the membership relation, so no two-write chain actually reaches
// membership here. The refusal fires anyway, because it checks the
// necessary condition rather than searching for a reachable chain.
//
// That is DELIBERATE, not a bug: computing reachability would put a graph
// walk with cycle handling inside a load path, and the policies this
// over-refuses already carry a High `rela acl audit` finding
// (A2-ungated-role-relation) whose fix is one requires_permission line.
// If this test ever "fails" because someone made the check smarter, the
// question to answer first is whether the smarter check can fail OPEN.
func TestRefusal_OverRefusesUnreachableChain_Deliberately(t *testing.T) {
	t.Parallel()
	p := worldPolicy(t,
		map[string]acl.RoleDef{
			"tagger":   {Update: []string{"tag"}, Read: []string{"tag"}},
			"everyone": {Read: []string{"world:published"}},
			"admin":    {Permissions: []string{"delegate-admin"}},
		},
		map[string]string{"admins": "admin"},
		map[string]acl.RoleRelationDef{
			"member-of": {RequiresPermission: "delegate-admin"},
			// tagger holds no permission at all, so self-granting it can
			// never pass the membership gate. Refused regardless.
			"tagged-by": {Confers: "tagger"},
		},
	)
	if err := p.Validate(); err == nil {
		t.Fatal("expected the deliberate over-refusal on an ungated privileged " +
			"role-relation, even though no chain reaches the membership gate")
	}
}

// TestMembershipSelfPromotionOpen_UnchangedByWorlds pins that the ADVISORY
// predicate keeps its TKT-T31NKT meaning after the world term is added to
// the REFUSAL. A1, A2, A3 and the boot warning all key on IsPrivileged, and
// they must not start firing on a world-only role — that would change what
// three existing audit findings report for policies doing nothing wrong.
//
// The policy is Validate()d first so `Worlds` is actually populated;
// membership is gated so the refusal does not fire and mask the assertion.
func TestMembershipSelfPromotionOpen_UnchangedByWorlds(t *testing.T) {
	t.Parallel()
	p := worldPolicy(t,
		map[string]acl.RoleDef{
			"reader": {Read: []string{"page", "world:published"}},
			"admin":  {Permissions: []string{"delegate-admin"}},
		},
		map[string]string{"readers": "reader"},
		map[string]acl.RoleRelationDef{
			"member-of": {RequiresPermission: "delegate-admin"},
		},
	)
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load (membership is gated): %v", err)
	}
	if got := p.Roles["reader"].Worlds; len(got) != 1 || got[0] != "published" {
		t.Fatalf("precondition: the world grant must be split out; got %v", got)
	}
	if p.Roles["reader"].IsPrivileged() {
		t.Error("IsPrivileged must NOT count a world grant — widening it would " +
			"change what A1/A2/A3 and the boot warning report")
	}
	if p.MembershipSelfPromotionOpen() {
		t.Error("MembershipSelfPromotionOpen must stay privilege-gated in the " +
			"IsPrivileged sense — a read-only role is not an A1 finding")
	}
}
