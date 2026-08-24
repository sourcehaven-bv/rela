package acl

import "testing"

// TestPolicy_MembershipSelfPromotionOpen pins the shared predicate behind both
// the aclaudit A1-ungated-membership finding and the boot-time startup warning
// (TKT-T31NKT). The privilege gate is the load-bearing part: widening it to
// "any assignment" would fire on read-only groups, the false positive the
// audit design explicitly fought (RR-EG5D3E).
func TestPolicy_MembershipSelfPromotionOpen(t *testing.T) {
	t.Parallel()

	privileged := RoleDef{Create: []string{"ticket"}, Read: []string{"ticket"}}
	readOnly := RoleDef{Read: []string{"ticket"}}

	tests := []struct {
		name   string
		policy Policy
		want   bool
	}{
		{
			name: "ungated default member-of with privileged assignment",
			policy: Policy{
				Roles:       map[string]RoleDef{"editor": privileged},
				Assignments: map[string]string{"engineering": "editor"},
			},
			want: true,
		},
		{
			name: "gated by requires_permission",
			policy: Policy{
				Roles:         map[string]RoleDef{"editor": privileged},
				Assignments:   map[string]string{"engineering": "editor"},
				RoleRelations: map[string]RoleRelationDef{"member-of": {RequiresPermission: "delegate-membership"}},
			},
			want: false,
		},
		{
			name: "read-only assigned role is a visibility choice, not escalation",
			policy: Policy{
				Roles:       map[string]RoleDef{"reader": readOnly},
				Assignments: map[string]string{"engineering": "reader"},
			},
			want: false,
		},
		{
			name: "assignment naming an undeclared role confers nothing",
			policy: Policy{
				Roles:       map[string]RoleDef{"editor": privileged},
				Assignments: map[string]string{"engineering": "ghost"},
			},
			want: false,
		},
		{
			name:   "no assignments at all",
			policy: Policy{Roles: map[string]RoleDef{"editor": privileged}},
			want:   false,
		},
		{
			name: "permission-only role counts as privileged",
			policy: Policy{
				Roles:       map[string]RoleDef{"delegator": {Permissions: []string{"delegate-membership"}}},
				Assignments: map[string]string{"admins": "delegator"},
			},
			want: true,
		},
		{
			// The gate must be read through EffectiveMembershipRelation, so a
			// configured relation is checked against its OWN role_relations
			// entry — not the default member-of.
			name: "configured relation, ungated (default member-of gate does not count)",
			policy: Policy{
				MembershipRelation: "heeft_rol",
				Roles:              map[string]RoleDef{"editor": privileged},
				Assignments:        map[string]string{"engineering": "editor"},
				RoleRelations:      map[string]RoleRelationDef{"member-of": {RequiresPermission: "delegate-membership"}},
			},
			want: true,
		},
		{
			name: "configured relation, gated on the configured name",
			policy: Policy{
				MembershipRelation: "heeft_rol",
				Roles:              map[string]RoleDef{"editor": privileged},
				Assignments:        map[string]string{"engineering": "editor"},
				RoleRelations:      map[string]RoleRelationDef{"heeft_rol": {RequiresPermission: "delegate-membership"}},
			},
			want: false, // gated on the effective relation → closed
		},
		{
			// Whitespace in membership_relation resolves through the same
			// trimming accessor the resolver walks, so the gate is found.
			name: "whitespace-padded relation name still finds its gate",
			policy: Policy{
				MembershipRelation: " heeft_rol ",
				Roles:              map[string]RoleDef{"editor": privileged},
				Assignments:        map[string]string{"engineering": "editor"},
				RoleRelations:      map[string]RoleRelationDef{"heeft_rol": {RequiresPermission: "delegate-membership"}},
			},
			want: false,
		},
		{
			name: "one privileged assignment among several read-only ones is enough",
			policy: Policy{
				Roles: map[string]RoleDef{"reader": readOnly, "editor": privileged},
				Assignments: map[string]string{
					"support":     "reader",
					"engineering": "editor",
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.policy.MembershipSelfPromotionOpen(); got != tc.want {
				t.Errorf("MembershipSelfPromotionOpen() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRoleDef_IsPrivileged pins the definition of "privileged" that the
// membership predicate and the aclaudit A2/A3 checks share. Read grants are
// never privilege (RR-LXI3NW / RR-UR0LJU).
func TestRoleDef_IsPrivileged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role RoleDef
		want bool
	}{
		{"empty role", RoleDef{}, false},
		{"read only", RoleDef{Read: []string{"ticket"}}, false},
		{"read wildcard is not privilege", RoleDef{Read: []string{"*"}}, false},
		{"create", RoleDef{Create: []string{"ticket"}}, true},
		{"update", RoleDef{Update: []string{"ticket"}}, true},
		{"delete", RoleDef{Delete: []string{"ticket"}}, true},
		{"permissions only", RoleDef{Permissions: []string{"delegate-membership"}}, true},
		{"write wildcard", RoleDef{Create: []string{"*"}}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.role.IsPrivileged(); got != tc.want {
				t.Errorf("IsPrivileged() = %v, want %v", got, tc.want)
			}
		})
	}
}
