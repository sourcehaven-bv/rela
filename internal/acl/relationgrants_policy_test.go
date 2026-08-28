package acl

import (
	"strings"
	"testing"
)

// TestRelationGrants_RejectsRoleConferringTypes is the load-bearing half of the
// delegate-X hardening at LOAD time.
//
// A relation permission satisfies the source-type verb grant, which is the only
// thing standing between a principal and a self-granted role edge. Checking
// only role_relations would miss two mechanisms — and the miss is the RR-7O6Q
// attack verbatim, since the membership relation need not appear in
// role_relations at all.
func TestRelationGrants_RejectsRoleConferringTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "default membership relation",
			body: `
roles: {admin: {read: ["*"]}}
assignments: {admins: admin}
relation_grants:
  member-of:
    create: link-membership
`,
			want: "membership relation",
		},
		{
			name: "configured membership relation",
			body: `
membership_relation: heeft_rol
roles: {admin: {read: ["*"]}}
relation_grants:
  heeft_rol:
    create: link-membership
`,
			want: "membership relation",
		},
		{
			name: "inherit_roles_through",
			body: `
inherit_roles_through: [contained-in]
roles: {admin: {read: ["*"]}}
relation_grants:
  contained-in:
    create: link-containers
`,
			want: "inherit_roles_through",
		},
		{
			name: "gated role-relation",
			body: `
roles: {admin: {read: ["*"]}}
role_relations:
  owner-of:
    confers: owner
    requires_permission: delegate-ownership
relation_grants:
  owner-of:
    create: link-owner
`,
			want: "requires_permission",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := LoadPolicyBytes([]byte(tc.body))
			if err == nil {
				t.Fatal("policy loaded; a relation permission on a role-conferring " +
					"type hands over the self-promotion primitive (RR-7O6Q)")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// TestRelationGrants_AllowsOrdinaryRelationTypes is the counterweight: the
// checks above must not reject the normal case.
func TestRelationGrants_AllowsOrdinaryRelationTypes(t *testing.T) {
	t.Parallel()
	p, err := LoadPolicyBytes([]byte(`
membership_relation: heeft_rol
inherit_roles_through: [contained-in]
roles: {sched: {read: ["*"], permissions: [create-spawnt]}}
role_relations:
  owner-of: {confers: owner}
relation_grants:
  spawnt:
    create: create-spawnt
  owner-of:
    create: link-owner
`))
	if err != nil {
		t.Fatalf("ordinary relation types rejected: %v", err)
	}
	if got, ok := p.relationPermissionFor("spawnt", OpCreate); !ok || got != "create-spawnt" {
		t.Errorf("relationPermissionFor(spawnt, create) = %q,%v", got, ok)
	}
	// An UNGATED role-relation is allowed here: it confers a role, but with no
	// requires_permission there is no delegate gate for the grant to undercut,
	// and `rela acl audit` already flags that shape (A2) as the real problem.
	if _, ok := p.relationPermissionFor("owner-of", OpCreate); !ok {
		t.Error("ungated role-relation should be grantable")
	}
}

func TestRelationGrants_StructuralValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "blank relation key",
			body: "relation_grants:\n  \"  \":\n    create: p\n",
			want: "must not be empty",
		},
		{
			name: "shorthand and per-verb are exclusive",
			body: "relation_grants:\n  spawnt:\n    permission: manage\n    delete: remove\n",
			want: "mutually exclusive",
		},
		{
			name: "grants nothing",
			body: "relation_grants:\n  spawnt: {}\n",
			want: "grants nothing",
		},
		{
			name: "blank permission is not a grant",
			body: "relation_grants:\n  spawnt:\n    create: \"   \"\n",
			want: "grants nothing",
		},
		{
			name: "read is refused with an explanation",
			body: "relation_grants:\n  spawnt:\n    read: see-spawnt\n",
			want: "not supported",
		},
		{
			name: "unknown verb",
			body: "relation_grants:\n  spawnt:\n    rename: p\n",
			want: "unknown key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := LoadPolicyBytes([]byte(tc.body))
			if err == nil {
				t.Fatal("policy loaded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

// TestRelationGrants_ReadErrorExplainsWhy pins the wording, not just the
// rejection. An operator writing `read:` has a coherent intent, so "unknown
// key" would read as an oversight and invite a workaround; the message has to
// carry the reason, because it reaches them at the moment they are wrong.
func TestRelationGrants_ReadErrorExplainsWhy(t *testing.T) {
	t.Parallel()
	_, err := LoadPolicyBytes([]byte("relation_grants:\n  spawnt:\n    read: see-spawnt\n"))
	if err == nil {
		t.Fatal("want error")
	}
	got := strings.ToLower(err.Error())
	for _, want := range []string{"both", "endpoints", "hide an endpoint"} {
		if !strings.Contains(got, want) {
			t.Errorf("error %q does not explain %q", err, want)
		}
	}
}

// TestRelationGrants_Normalization pins that padded keys and permissions are
// trimmed rather than loading clean and never matching anything.
func TestRelationGrants_Normalization(t *testing.T) {
	t.Parallel()
	p, err := LoadPolicyBytes([]byte(
		"roles: {r: {read: [\"*\"]}}\nrelation_grants:\n  \"  spawnt  \":\n    create: \"  create-spawnt  \"\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := p.relationPermissionFor("spawnt", OpCreate)
	if !ok {
		t.Fatal("padded relation-type key did not normalize; the entry would be inert")
	}
	if got != "create-spawnt" {
		t.Errorf("permission = %q, want create-spawnt (untrimmed would never match)", got)
	}
}

// TestRelationWriteGrant_ShorthandCoversWriteVerbsOnly pins that the shorthand
// does not silently pick up OpRename via the Update routing grantsVerb uses.
func TestRelationWriteGrant_ShorthandCoversWriteVerbsOnly(t *testing.T) {
	t.Parallel()
	g := RelationWriteGrant{Permission: "manage"}
	for _, op := range []Op{OpCreate, OpUpdate, OpDelete} {
		if perm, ok := g.permissionFor(op); !ok || perm != "manage" {
			t.Errorf("permissionFor(%s) = %q,%v; want manage,true", op, perm, ok)
		}
	}
	if perm, ok := g.permissionFor(OpRename); ok {
		t.Errorf("permissionFor(rename) = %q,true; no caller pairs OpRename with a "+
			"RelationSubject, so accepting it invents a semantic no path exercises", perm)
	}
}
