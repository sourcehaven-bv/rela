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
