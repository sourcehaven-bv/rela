package aclaudit

import (
	"fmt"
	"slices"
	"sort"
	"strings"

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
	f = append(f, checkUndeclaredWorlds(p, m)...)         // B10
	f = append(f, checkUndeclaredFaces(p, m)...)          // B11
	return f
}

// B10 — a `read: [world:X]` grant names a world the metamodel does not
// declare, so it grants access to nothing.
//
// This is the cross-file half of the world grant syntax. internal/acl
// validates the SPELLING at load (non-empty, no glob) but structurally
// cannot check EXISTENCE — arch-lint forbids it a metamodel dependency, so
// a world name is just a string there. The runtime is already fail-closed
// on an unknown world (worlds.Compiled.Lookup returns ok=false), which
// means the failure mode is a silent denial rather than a leak: exactly the
// shape an operator needs a linter to explain.
//
// The implicit DEFAULT world is accepted without being declared — it is
// total, always exists, and `read: [world:default]` is a legal way to spell
// what a bare read grant already means.
func checkUndeclaredWorlds(p *acl.Policy, m MetamodelReader) []Finding {
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		role := p.Roles[name]
		seen := map[string]bool{}
		// Worlds is populated only by the validating load. A caller that
		// skipped Validate still has its world tokens inline in Read, where
		// B1 would report them as undeclared ENTITY TYPES — misleading
		// advice at High severity. Scanning both means the diagnosis is
		// right either way. Audit's godoc still states the precondition.
		worlds := role.Worlds
		for _, entry := range role.Read {
			if w, found := strings.CutPrefix(entry, acl.WorldGrantPrefix); found {
				worlds = append(worlds, strings.TrimSpace(w))
			}
		}
		for _, world := range worlds {
			if world == acl.DefaultWorldName || m.HasWorld(world) || seen[world] {
				continue
			}
			seen[world] = true
			if f2, isCaseVariant := defaultWorldCaseVariant(name, world); isCaseVariant {
				f = append(f, f2)
				continue
			}
			f = append(f, Finding{
				Rule: "B10-undeclared-world", Severity: High, Subject: name,
				Detail: fmt.Sprintf("role %q grants read on world %q which the metamodel does not "+
					"declare; the grant matches nothing and every read in that world is denied",
					name, world),
				Fix: fmt.Sprintf("declare world %q under `worlds:` in schema.yaml, or fix the "+
					"`read: [world:%s]` grant (check for a typo)", world, world),
			})
		}
	}
	return f
}

// defaultWorldCaseVariant reports a world grant that spells the default
// world with the wrong casing, and returns the finding that explains it.
//
// This case needs its own message because the ordinary B10 remedy is
// IMPOSSIBLE to follow: the metamodel loader rejects any world whose name
// case-folds to "default" as reserved, so telling the operator to declare
// `Default` sends them to a schema that will refuse to load. The grant is
// genuinely dead — roleGrantsWorldRead compares case-sensitively — so
// flagging it is right; only the fix differs.
func defaultWorldCaseVariant(role, world string) (Finding, bool) {
	if world == acl.DefaultWorldName || !strings.EqualFold(world, acl.DefaultWorldName) {
		return Finding{}, false
	}
	return Finding{
		Rule: "B10-undeclared-world", Severity: High, Subject: role,
		Detail: fmt.Sprintf("role %q grants read on world %q; the default world is spelled %q "+
			"in lowercase and cannot be redeclared under any other casing (the schema "+
			"loader rejects the name as reserved), so this grant matches nothing",
			role, world, acl.DefaultWorldName),
		Fix: fmt.Sprintf("write `read: [world:%s]` in lowercase, or drop the grant entirely "+
			"— an ordinary read grant already covers the default world",
			acl.DefaultWorldName),
	}, true
}

// B11 — a `type@face` write grant names a content state the type does
// not declare, so it grants write access to nothing.
//
// Same division of labor as B10: internal/acl parses the face NAME
// against the codec grammar at load, but only the metamodel knows which
// faces a type actually declares. A grant on an undeclared state is
// fail-closed at runtime (no state row will ever match it), so again the
// symptom is a denial nobody can explain without this finding.
//
// Skips a type the metamodel does not declare at all — B1 reports that, and
// reporting both would give one mistake two findings with different fixes.
func checkUndeclaredFaces(p *acl.Policy, m MetamodelReader) []Finding {
	var f []Finding
	for _, name := range sortedRoleNames(p) {
		role := p.Roles[name]
		seen := map[string]bool{}
		for _, verb := range []string{"create", "update", "delete"} {
			for _, entry := range verbLists(role)[verb] {
				typeName, face, isState := splitStateGrant(entry)
				if !isState || seen[entry] {
					continue
				}
				// The type WILDCARD cannot carry a state. `*` ranges over
				// types and grants each one's DEFAULT state
				// (acl.GrantsVerbOnState honors it only for the zero
				// face), so `*@draft` matches an entity whose type is
				// literally named "*" and nothing else. It grants nothing,
				// loads clean, and would otherwise be reported by NOTHING:
				// B1 skips "*" as a wildcard and the HasEntityType gate
				// below skips it as undeclared.
				if typeName == "*" {
					seen[entry] = true
					f = append(f, Finding{
						Rule: "B11-undeclared-face", Severity: High, Subject: name,
						Detail: fmt.Sprintf("role %q grants %s on %q, but %q ranges over entity "+
							"TYPES and only ever grants each type's default state; it never "+
							"addresses a named content state, so this grant matches nothing",
							name, verb, entry, "*"),
						Fix: fmt.Sprintf("name each type explicitly (e.g. `page@%s`), or drop "+
							"the `@%s` to grant the default state of every type",
							face, face),
					})
					continue
				}
				if !m.HasEntityType(typeName) {
					continue
				}
				if bare := m.BareFace(typeName); bare != "" && face == bare {
					// Declared, so B11 is silent — but a grant names a face AS
					// STORED, and the bare face is stored under the bare id.
					// `update: [policy@draft]` with `bare_face: draft` matches
					// no row, ever; the grant that reaches that face is the
					// bare `update: [policy]`. Fail-closed, and inexplicable
					// without this finding.
					seen[entry] = true
					f = append(f, Finding{
						Rule: "B12-bare-face-named", Severity: High, Subject: name,
						Detail: fmt.Sprintf("role %q grants %s on %q, but %q is entity type %q's "+
							"bare face, which is addressed by the bare id — the grant matches nothing",
							name, verb, entry, face, typeName),
						Fix: fmt.Sprintf("write `%s: [%s]`: the bare face is stored under the bare id, "+
							"so the bare type grant is the one that reaches it",
							verb, typeName),
					})
					continue
				}
				if m.HasFace(typeName, face) {
					continue
				}
				seen[entry] = true
				f = append(f, Finding{
					Rule: "B11-undeclared-face", Severity: High, Subject: name,
					Detail: fmt.Sprintf("role %q grants %s on %q but entity type %q declares no "+
						"content state %q; the grant matches nothing",
						name, verb, entry, typeName, face),
					Fix: fmt.Sprintf("fix the %s grant (check for a typo), or declare %q under "+
						"the `faces:` block of entity type %q in schema.yaml — note that "+
						"adding a `faces:` block to a type that has none makes every entity "+
						"of that type subject to world resolution, so a plain `%s: [%s]` grant "+
						"is the likelier fix",
						verb, face, typeName, verb, typeName),
				})
			}
		}
	}
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
			// A world token is never an entity type. It should already have
			// been split out of Read by the validating load; if the caller
			// skipped Validate it is still here, and reporting it as an
			// undeclared TYPE would send the operator to declare
			// `world:published` in `entities:`. B10 diagnoses it properly.
			if strings.HasPrefix(entry, acl.WorldGrantPrefix) {
				return
			}
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
