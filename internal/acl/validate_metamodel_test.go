package acl_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// fakeMeta is a minimal acl.MetamodelView for validation tests. types maps
// entity type → property name → its PropertyInfo.
type fakeMeta struct {
	types map[string]map[string]acl.PropertyInfo
}

func (m fakeMeta) HasEntityType(t string) bool {
	_, ok := m.types[t]
	return ok
}

func (m fakeMeta) PropertyInfo(t, prop string) acl.PropertyInfo {
	props, ok := m.types[t]
	if !ok {
		return acl.PropertyInfo{}
	}
	return props[prop] // zero value (Exists:false) when absent
}

func TestValidateAgainstMetamodel(t *testing.T) {
	meta := fakeMeta{types: map[string]map[string]acl.PropertyInfo{
		"persoon": {
			"email":    {Exists: true, Unique: true},
			"nickname": {Exists: true, Unique: false},
			"aliases":  {Exists: true, Unique: true, List: true},
		},
	}}

	cases := []struct {
		name       string
		policy     acl.Policy
		wantErr    bool
		wantSubstr string
	}{
		{
			name:   "valid: unique property",
			policy: acl.Policy{UserEntityType: "persoon", PrincipalProperty: "email"},
		},
		{
			name:   "valid: user_entity_type alone (no principal_property)",
			policy: acl.Policy{UserEntityType: "persoon"},
		},
		{
			name:   "valid: neither set",
			policy: acl.Policy{},
		},
		{
			name:       "principal_property without user_entity_type",
			policy:     acl.Policy{PrincipalProperty: "email"},
			wantErr:    true,
			wantSubstr: "requires user_entity_type",
		},
		{
			name:       "unknown entity type",
			policy:     acl.Policy{UserEntityType: "bogus", PrincipalProperty: "email"},
			wantErr:    true,
			wantSubstr: "not a declared entity type",
		},
		{
			name:       "unknown property",
			policy:     acl.Policy{UserEntityType: "persoon", PrincipalProperty: "ghost"},
			wantErr:    true,
			wantSubstr: "not a declared property",
		},
		{
			name:       "non-unique property",
			policy:     acl.Policy{UserEntityType: "persoon", PrincipalProperty: "nickname"},
			wantErr:    true,
			wantSubstr: "must be declared `unique: true`",
		},
		{
			name:       "list property rejected even when unique",
			policy:     acl.Policy{UserEntityType: "persoon", PrincipalProperty: "aliases"},
			wantErr:    true,
			wantSubstr: "list: true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.ValidateAgainstMetamodel(meta)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantSubstr)
				}
				if !strings.Contains(err.Error(), tc.wantSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAgainstMetamodel_NilView(t *testing.T) {
	p := acl.Policy{UserEntityType: "persoon", PrincipalProperty: "email"}
	if err := p.ValidateAgainstMetamodel(nil); err == nil {
		t.Fatal("expected error for nil metamodel view")
	}
}

// TestValidateAgainstMetamodel_ProvisionRequiresGrant pins TKT-ANUJDS AC4: a
// provision policy must grant system:provisioner create on the user type, or
// the whole policy fails to load. Non-provision policies are never subject to
// the check.
func TestValidateAgainstMetamodel_ProvisionRequiresGrant(t *testing.T) {
	meta := fakeMeta{types: map[string]map[string]acl.PropertyInfo{
		"persoon": {"email": {Exists: true, Unique: true}},
	}}
	base := func() acl.Policy {
		return acl.Policy{
			UserEntityType:     "persoon",
			PrincipalProperty:  "email",
			UnmatchedPrincipal: acl.UnmatchedProvision,
		}
	}

	cases := []struct {
		name       string
		mutate     func(*acl.Policy)
		wantErr    bool
		wantSubstr string
	}{
		{
			name:       "provision without any grant fails",
			mutate:     func(*acl.Policy) {},
			wantErr:    true,
			wantSubstr: "requires \"system:provisioner\"",
		},
		{
			name: "provision with assignments grant on the user type",
			mutate: func(p *acl.Policy) {
				p.Roles = map[string]acl.RoleDef{"prov": {Create: []string{"persoon"}}}
				p.Assignments = map[string]string{"system:provisioner": "prov"}
			},
		},
		{
			name: "provision with wildcard create grant",
			mutate: func(p *acl.Policy) {
				p.Roles = map[string]acl.RoleDef{"prov": {Create: []string{"*"}}}
				p.Assignments = map[string]string{"system:provisioner": "prov"}
			},
		},
		{
			name: "provision with asserted-role grant",
			mutate: func(p *acl.Policy) {
				p.Roles = map[string]acl.RoleDef{"prov": {Create: []string{"persoon"}}}
				p.AssertedRoles = map[string]acl.RoleList{"system:provisioner": {"prov"}}
			},
		},
		{
			name: "grant creates the WRONG type fails",
			mutate: func(p *acl.Policy) {
				p.Roles = map[string]acl.RoleDef{"prov": {Create: []string{"ticket"}}}
				p.Assignments = map[string]string{"system:provisioner": "prov"}
			},
			wantErr:    true,
			wantSubstr: "requires \"system:provisioner\"",
		},
		{
			name: "assignment names an undefined role fails",
			mutate: func(p *acl.Policy) {
				p.Assignments = map[string]string{"system:provisioner": "ghost"}
			},
			wantErr:    true,
			wantSubstr: "requires \"system:provisioner\"",
		},
		{
			name: "reject policy never needs the grant",
			mutate: func(p *acl.Policy) {
				p.UnmatchedPrincipal = acl.UnmatchedReject
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base()
			tc.mutate(&p)
			err := p.ValidateAgainstMetamodel(meta)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantSubstr)
				}
				if !strings.Contains(err.Error(), tc.wantSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
