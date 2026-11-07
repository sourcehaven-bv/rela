package acl_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// TestValidateAgainstMetamodel_VisibleAliasKey pins issue #2106 on the ACL
// side: a `visible:` or `redact:` key that names a type through an alias
// would restrict nothing, so it fails to load, and the canonical key still
// restricts the field the flow guard check asks about.
func TestValidateAgainstMetamodel_VisibleAliasKey(t *testing.T) {
	meta := fakeMeta{
		types: map[string]map[string]acl.PropertyInfo{
			"person": {"salary": {Exists: true}, "name": {Exists: true}},
		},
		aliases: map[string]string{"pers": "person"},
	}
	cases := []struct {
		name   string
		policy acl.Policy
		want   string
	}{
		{
			name: "role visible keyed by alias",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{
				"hr": {Read: []string{"person"}, Visible: map[string][]acl.FieldGrant{"pers": {{Field: "name"}}}},
			}},
			want: `roles.hr.visible: "pers" names the type through an alias; use the canonical type name: "person"`,
		},
		{
			name: "baseline redact keyed by alias",
			policy: acl.Policy{ClientBaselines: map[string]acl.ClientBaseline{
				"apps": {AppliesTo: []string{"app"}, Restriction: acl.Restriction{
					Redact: map[string][]string{"pers": {"salary"}},
				}},
			}},
			want: `client_baselines.apps.redact: "pers"`,
		},
		{
			name: "canonical key loads",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{
				"hr": {Read: []string{"person"}, Visible: map[string][]acl.FieldGrant{"person": {{Field: "name"}}}},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.policy
			if err := p.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			err := p.ValidateAgainstMetamodel(meta)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("ValidateAgainstMetamodel: %v", err)
				}
				if !p.FieldRestricted("person", "salary") {
					t.Error("a canonical visible: key does not restrict salary on person")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateAgainstMetamodel = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}
