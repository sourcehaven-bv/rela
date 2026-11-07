package acl

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// validateVisibleAliasKeys refuses a `visible:` or `redact:` key that names
// a type through an alias (issue #2106). Keys are matched against canonical
// type names literally ([grantKeyApplies]), so an alias key would load
// clean and restrict nothing: the role would read every property, and the
// flow guard check ([Policy.FieldRestricted]) would not see the restriction
// either. The message names the canonical form.
func validateVisibleAliasKeys(p *Policy, meta MetamodelView) []error {
	var errs []error
	check := func(where string, keys []string) {
		slices.Sort(keys)
		for _, key := range keys {
			if key == "*" {
				continue
			}
			typeName, face, faced := strings.Cut(key, entity.StateRefSeparator)
			canonical, _ := meta.FaceNames(typeName)
			if canonical == "" || canonical == typeName {
				continue
			}
			want := canonical
			if faced {
				want += entity.StateRefSeparator + face
			}
			errs = append(errs, fmt.Errorf("acl: %s: %q names the type through an alias; "+
				"use the canonical type name: %q", where, key, want))
		}
	}
	for _, name := range sortedRoleNames(p.Roles) {
		check("roles."+name+".visible", slices.Collect(maps.Keys(p.Roles[name].Visible)))
	}
	for _, name := range slices.Sorted(maps.Keys(p.ClientBaselines)) {
		b := p.ClientBaselines[name]
		check("client_baselines."+name+".visible", slices.Collect(maps.Keys(b.Visible)))
		check("client_baselines."+name+".redact", slices.Collect(maps.Keys(b.Redact)))
	}
	return errs
}
