package aclaudit

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// tierB runs the metamodel cross-checks: grants and relation names that the
// schema doesn't declare (silent drift). All checks skip the "*" wildcard
// sentinel, which is not an entity type (RR-TZ2S3G).
func tierB(p *acl.Policy, m MetamodelReader) []Finding {
	f := make([]Finding, 0, 8)
	f = append(f, checkUndeclaredEntityTypes(p, m)...)    // B1
	f = append(f, checkUndeclaredRelations(p, m)...)      // B2
	f = append(f, checkMembershipFromCompat(p, m)...)     // B3
	f = append(f, checkUndeclaredFields(p, m)...)         // B4
	f = append(f, checkUndeclaredOptions(p, m)...)        // B5
	f = append(f, checkUserEntityTypeDeclared(p, m)...)   // B7
	f = append(f, checkCeilingsAgainstMetamodel(p, m)...) // B8 / B9
	return f
}

// B1 — a create/update/delete/read grant or an affordance-map key names an
// entity type the metamodel doesn't declare: the grant silently matches
// nothing. The "*" wildcard is skipped (it's not a type).
func checkUndeclaredEntityTypes(p *acl.Policy, m MetamodelReader) []Finding {
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		role := p.Roles[name]
		seen := map[string]bool{}
		flag := func(verb, entry string) {
			// Check the TYPE half. A write grant may be state-shaped
			// (`page@draft`, TKT-DN37J2); the type is what the metamodel
			// declares, and reporting the joined string as an undeclared
			// type would fail CI on every correct state grant (this finding
			// is High, and `rela acl audit --exit-code` gates on High).
			// Whether the POINTER is declared is a separate check.
			t := grantEntityType(entry)
			// Dedupe on type alone: one typo'd type used across several verbs
			// (or an affordance key) is one mistake with one fix.
			if t == "*" || m.HasEntityType(t) || seen[t] {
				return
			}
			seen[t] = true
			f = append(f, Finding{
				Rule: "B1-undeclared-type", Severity: High, Subject: name,
				Detail: fmt.Sprintf("role %q grants %s on entity type %q which the metamodel does not "+
					"declare; the grant matches nothing", name, verb, t),
				Fix: fmt.Sprintf("declare entity type %q, or fix the %s grant (check for a typo)", t, verb),
			})
		}
		for _, verb := range []string{"create", "update", "delete", "read"} {
			for _, t := range verbLists(role)[verb] {
				flag(verb, t)
			}
		}
		// Affordance map keys are entity types with no wildcard.
		for _, t := range affordanceTypeKeys(role) {
			flag("affordance", t)
		}
	}
	return f
}

// B2 — membership_relation, a role_relations key, or an inherit_roles_through
// entry names a relation type the metamodel doesn't declare: the resolver
// walks a relation that can't exist, so it confers nothing.
func checkUndeclaredRelations(p *acl.Policy, m MetamodelReader) []Finding {
	var f []Finding
	check := func(rel, kind string) {
		if rel == "" {
			return
		}
		if _, ok := m.GetRelation(rel); ok {
			return
		}
		f = append(f, Finding{
			Rule: "B2-undeclared-relation", Severity: High, Subject: rel,
			Detail: fmt.Sprintf("%s names relation type %q which the metamodel does not declare; the "+
				"resolver walks a relation that cannot exist", kind, rel),
			Fix: fmt.Sprintf("declare relation type %q, or fix the %s (check for a typo)", rel, kind),
		})
	}
	// Only flag membership_relation when explicitly set — an unset/default
	// member-of that the schema lacks is its own (common) situation, and B2
	// would be noise on a project that simply doesn't model member-of.
	if p.MembershipRelation != "" {
		check(p.EffectiveMembershipRelation(), "membership_relation")
	}
	for _, rel := range sortedRelTypes(p) {
		check(rel, "role_relations key")
	}
	inherited := append([]string(nil), p.InheritRolesThrough...)
	sort.Strings(inherited)
	for _, rel := range inherited {
		check(rel, "inherit_roles_through entry")
	}
	return f
}

// B3 — the membership relation's `from` (per the metamodel) does not include
// user_entity_type, so a user can never hold that edge: groups never resolve.
// Only meaningful when user_entity_type is set and the relation is declared.
func checkMembershipFromCompat(p *acl.Policy, m MetamodelReader) []Finding {
	if p.UserEntityType == "" {
		return nil
	}
	rel := p.EffectiveMembershipRelation()
	view, ok := m.GetRelation(rel)
	if !ok {
		return nil // B2 already reports an undeclared relation
	}
	if slices.Contains(view.From, p.UserEntityType) {
		return nil
	}
	return []Finding{{
		Rule: "B3-membership-from-mismatch", Severity: Medium, Subject: rel,
		Detail: fmt.Sprintf("membership relation %q has from=%v in the metamodel, which does not include "+
			"user_entity_type %q; a user can never hold this edge, so group membership never resolves",
			rel, view.From, p.UserEntityType),
		Fix: fmt.Sprintf("add %q to the from of relation %q, or set membership_relation to a relation "+
			"whose source is %q", p.UserEntityType, rel, p.UserEntityType),
	}}
}

// B4 — a fields/visible grant names a field the entity type doesn't declare.
func checkUndeclaredFields(p *acl.Policy, m MetamodelReader) []Finding {
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		role := p.Roles[name]
		for _, block := range []struct {
			kind   string
			grants map[string][]acl.FieldGrant
		}{{"fields", role.Fields}, {"visible", role.Visible}} {
			for _, t := range sortedKeysFieldGrant(block.grants) {
				if !m.HasEntityType(t) {
					continue // B1 reports the undeclared type; don't double-flag fields
				}
				for _, g := range block.grants[t] {
					if g.Field == "" || m.HasField(t, g.Field) {
						continue
					}
					f = append(f, Finding{
						Rule: "B4-undeclared-field", Severity: Medium, Subject: name,
						Detail: fmt.Sprintf("role %q %s grant on %q names field %q which the type does not "+
							"declare; the affordance does nothing", name, block.kind, t, g.Field),
						Fix: fmt.Sprintf("declare field %q on %q, or fix the grant", g.Field, t),
					})
				}
			}
		}
	}
	return f
}

// B5 — an options grant names a value outside the field's enum (or a
// non-enum field).
func checkUndeclaredOptions(p *acl.Policy, m MetamodelReader) []Finding {
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		role := p.Roles[name]
		for _, t := range sortedKeysOptionGrant(role.Options) {
			if !m.HasEntityType(t) {
				continue
			}
			for _, g := range role.Options[t] {
				if g.Field == "" {
					continue
				}
				// Distinguish "field doesn't exist" (a typo) from "field exists
				// but isn't an enum" — the operator needs the right diagnosis.
				if !m.HasField(t, g.Field) {
					f = append(f, Finding{
						Rule: "B4-undeclared-field", Severity: Medium, Subject: name,
						Detail: fmt.Sprintf("role %q options grant on %q names field %q which the type does "+
							"not declare; the grant does nothing", name, t, g.Field),
						Fix: fmt.Sprintf("declare field %q on %q, or fix the grant", g.Field, t),
					})
					continue
				}
				opts, isEnum := m.EnumOptions(t, g.Field)
				if !isEnum {
					f = append(f, Finding{
						Rule: "B5-options-non-enum", Severity: Medium, Subject: name,
						Detail: fmt.Sprintf("role %q options grant on %q names field %q which is not an enum; "+
							"the option grant does nothing", name, t, g.Field),
						Fix: fmt.Sprintf("grant options only on enum fields; %q.%q is not one", t, g.Field),
					})
					continue
				}
				if g.Option != "" && !slices.Contains(opts, g.Option) {
					f = append(f, Finding{
						Rule: "B5-undeclared-option", Severity: Medium, Subject: name,
						Detail: fmt.Sprintf("role %q options grant on %q.%q names option %q outside the field's "+
							"enum %v; the grant does nothing", name, t, g.Field, g.Option, opts),
						Fix: fmt.Sprintf("use one of %v, or fix the option (check for a typo)", opts),
					})
				}
			}
		}
	}
	return f
}

// B7 — user_entity_type names a type the metamodel doesn't declare: the
// whole user-resolution layer is broken.
func checkUserEntityTypeDeclared(p *acl.Policy, m MetamodelReader) []Finding {
	if p.UserEntityType == "" || m.HasEntityType(p.UserEntityType) {
		return nil
	}
	return []Finding{{
		Rule: "B7-undeclared-user-type", Severity: High, Subject: p.UserEntityType,
		Detail: fmt.Sprintf("user_entity_type %q is not a declared entity type; user resolution cannot "+
			"work", p.UserEntityType),
		Fix: fmt.Sprintf("declare entity type %q, or fix user_entity_type", p.UserEntityType),
	}}
}

// ---- helpers -----------------------------------------------------------

func sortedKeysFieldGrant(m map[string][]acl.FieldGrant) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysOptionGrant(m map[string][]acl.OptionGrant) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
