package acl_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// facedMeta declares one faced type (policy, alias pol), one faceless type
// (ticket), and the relations the identity-structure checks read.
func facedMeta() fakeMeta {
	return fakeMeta{
		types: map[string]map[string]acl.PropertyInfo{
			"policy": {}, "ticket": {}, "person": {}, "team": {}, "folder": {},
		},
		faces:   map[string][]string{"policy": {"draft", "published"}},
		aliases: map[string]string{"pol": "policy"},
	}
}

// TestValidateAgainstMetamodel_FacedWriteGrants pins TKT-7IZHP0 §7: a write
// grant naming a faced type without a face is a load error that names the
// grant and the face-qualified replacements. `*` and read grants are not
// affected.
func TestValidateAgainstMetamodel_FacedWriteGrants(t *testing.T) {
	cases := []struct {
		name     string
		role     acl.RoleDef
		wantErr  []string
		noSubstr []string
	}{
		{
			name: "bare update on a faced type",
			role: acl.RoleDef{Read: []string{"*"}, Update: []string{"policy"}},
			wantErr: []string{
				`roles.editor.update: "policy" names a type that declares faces`,
				`"policy@draft", "policy@published"`,
			},
			noSubstr: []string{"note:"},
		},
		{
			name:    "bare create and delete are refused too",
			role:    acl.RoleDef{Read: []string{"*"}, Create: []string{"policy"}, Delete: []string{"policy"}},
			wantErr: []string{"roles.editor.create:", "roles.editor.delete:"},
		},
		{
			name: "an alias is refused and the canonical name suggested",
			role: acl.RoleDef{Read: []string{"*"}, Update: []string{"pol"}},
			wantErr: []string{
				`roles.editor.update: "pol" names a type`,
				`"policy@draft", "policy@published"`,
			},
		},
		{
			name: "a wildcard beside the bare grant adds a note",
			role: acl.RoleDef{Read: []string{"*"}, Update: []string{"*", "policy"}},
			wantErr: []string{
				`roles.editor.update: "policy"`,
				`note: "*" in this list reaches only types without faces, so it does not cover "policy"`,
			},
		},
		{
			name:     "no note when the list also names a face",
			role:     acl.RoleDef{Read: []string{"*"}, Update: []string{"*", "policy", "policy@draft"}},
			wantErr:  []string{`roles.editor.update: "policy"`},
			noSubstr: []string{"note:"},
		},
		{
			name: "face-qualified grants load",
			role: acl.RoleDef{Read: []string{"*"}, Update: []string{"policy@draft", "policy@published"}},
		},
		{
			name: "the wildcard alone loads",
			role: acl.RoleDef{Read: []string{"*"}, Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"}},
		},
		{
			name: "a bare read grant on a faced type loads",
			role: acl.RoleDef{Read: []string{"policy"}},
		},
		{
			name: "a bare write grant on a faceless type loads",
			role: acl.RoleDef{Read: []string{"*"}, Update: []string{"ticket"}},
		},
		{
			name: "an undeclared type is left to aclaudit",
			role: acl.RoleDef{Read: []string{"*"}, Update: []string{"nosuch"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := acl.Policy{Roles: map[string]acl.RoleDef{"editor": tc.role}}
			err := p.ValidateAgainstMetamodel(facedMeta())
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			for _, bad := range tc.noSubstr {
				if strings.Contains(err.Error(), bad) {
					t.Errorf("error %q must not contain %q", err, bad)
				}
			}
		})
	}
}

// TestValidateAgainstMetamodel_ReportsEveryFacedGrant pins collect-then-report:
// every offending entry across roles is named in one error, in a stable order.
func TestValidateAgainstMetamodel_ReportsEveryFacedGrant(t *testing.T) {
	p := acl.Policy{Roles: map[string]acl.RoleDef{
		"b-role": {Read: []string{"*"}, Update: []string{"policy"}},
		"a-role": {Read: []string{"*"}, Create: []string{"policy", "policy"}, Delete: []string{"pol"}},
	}}
	err := p.ValidateAgainstMetamodel(facedMeta())
	if err == nil {
		t.Fatal("expected an error")
	}
	lines := strings.Split(err.Error(), "\n")
	want := []string{"roles.a-role.create:", "roles.a-role.delete:", "roles.b-role.update:"}
	if len(lines) != len(want) {
		t.Fatalf("got %d errors, want %d (a duplicate entry is reported once):\n%v", len(lines), len(want), err)
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Errorf("error %d = %q, want it to name %q", i, lines[i], w)
		}
	}
}

// TestValidateAgainstMetamodel_IdentityStructure pins D7 and A16: the ACL
// resolves identities and roles over tail-less edges, so a faced user,
// member, group or role-holder type, and a content-scoped relation the
// resolver walks, are load errors.
func TestValidateAgainstMetamodel_IdentityStructure(t *testing.T) {
	faced := map[string][]string{
		"fperson": {"draft", "published"},
		"fteam":   {"draft"},
		"policy":  {"draft", "published"},
	}
	types := map[string]map[string]acl.PropertyInfo{
		"person": {}, "fperson": {}, "team": {}, "fteam": {}, "policy": {}, "folder": {},
	}
	cases := []struct {
		name    string
		policy  acl.Policy
		rels    map[string]acl.RelationInfo
		wantErr []string
	}{
		{
			name:    "faced user type",
			policy:  acl.Policy{UserEntityType: "fperson"},
			wantErr: []string{`user_entity_type "fperson" declares faces (draft, published)`},
		},
		{
			name:   "faceless user type loads",
			policy: acl.Policy{UserEntityType: "person"},
		},
		{
			name:    "faced group type on the default membership relation",
			policy:  acl.Policy{},
			rels:    map[string]acl.RelationInfo{"member-of": {From: []string{"person"}, To: []string{"fteam"}}},
			wantErr: []string{`membership_relation "member-of": group type "fteam" declares faces`},
		},
		{
			name:    "faced member type on a configured membership relation",
			policy:  acl.Policy{MembershipRelation: "heeft_rol"},
			rels:    map[string]acl.RelationInfo{"heeft_rol": {From: []string{"fperson"}, To: []string{"team"}}},
			wantErr: []string{`membership_relation "heeft_rol": member type "fperson" declares faces`},
		},
		{
			name:   "content-scoped membership relation",
			policy: acl.Policy{},
			rels: map[string]acl.RelationInfo{
				"member-of": {Content: true, From: []string{"person"}, To: []string{"team"}},
			},
			wantErr: []string{`membership_relation "member-of": relation "member-of" is declared ` + "`scope: content`"},
		},
		{
			name:   "an undeclared membership relation is skipped",
			policy: acl.Policy{},
		},
		{
			name: "content-scoped role relation",
			policy: acl.Policy{
				Roles:         map[string]acl.RoleDef{"owner": {}},
				RoleRelations: map[string]acl.RoleRelationDef{"owns": {Confers: "owner"}},
			},
			rels: map[string]acl.RelationInfo{
				"owns": {Content: true, From: []string{"person"}, To: []string{"policy"}},
			},
			wantErr: []string{`role_relations.owns: relation "owns" is declared ` + "`scope: content`"},
		},
		{
			name: "faced role holder",
			policy: acl.Policy{
				Roles:         map[string]acl.RoleDef{"owner": {}},
				RoleRelations: map[string]acl.RoleRelationDef{"owns": {Confers: "owner"}},
			},
			rels:    map[string]acl.RelationInfo{"owns": {From: []string{"fteam"}, To: []string{"folder"}}},
			wantErr: []string{`role_relations.owns: role holder type "fteam" declares faces`},
		},
		{
			name: "a faced role TARGET loads: the role covers the whole entity",
			policy: acl.Policy{
				Roles:         map[string]acl.RoleDef{"owner": {}},
				RoleRelations: map[string]acl.RoleRelationDef{"owns": {Confers: "owner"}},
			},
			rels: map[string]acl.RelationInfo{"owns": {From: []string{"person"}, To: []string{"policy"}}},
		},
		{
			name: "a write gate without confers is not walked, so is not checked",
			policy: acl.Policy{
				RoleRelations: map[string]acl.RoleRelationDef{"cites": {RequiresPermission: "x"}},
			},
			rels: map[string]acl.RelationInfo{
				"cites": {Content: true, From: []string{"policy"}, To: []string{"policy"}},
			},
		},
		{
			name:   "content-scoped containment relation",
			policy: acl.Policy{InheritRolesThrough: []string{"in-folder"}},
			rels: map[string]acl.RelationInfo{
				"in-folder": {Content: true, From: []string{"policy"}, To: []string{"folder"}},
			},
			wantErr: []string{`inherit_roles_through "in-folder": relation "in-folder" is declared ` + "`scope: content`"},
		},
		{
			name:   "identity-scoped containment of a faced child loads",
			policy: acl.Policy{InheritRolesThrough: []string{"in-folder"}},
			rels:   map[string]acl.RelationInfo{"in-folder": {From: []string{"policy"}, To: []string{"folder"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := fakeMeta{types: types, faces: faced, relInfo: tc.rels}
			err := tc.policy.ValidateAgainstMetamodel(meta)
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}
