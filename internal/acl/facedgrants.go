package acl

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// RelationInfo describes a relation type to [Policy.ValidateAgainstMetamodel].
// Exists is false when the schema does not declare the type; the other fields
// are then meaningless.
type RelationInfo struct {
	Exists bool
	// Content is true for `scope: content`: the edge attaches to one face of
	// its source rather than to the entity's identity.
	Content bool
	// From and To are the declared endpoint types, as written in the schema.
	From []string
	To   []string
}

// validateFacedWriteGrants refuses a `create`, `update` or `delete` entry that
// names a faced type without a face (TKT-7IZHP0 §7).
//
// A bare grant addresses the implicit face, and a faced type stores no row
// there, so the grant authorizes nothing. It used to load clean and fail as a
// denial with no visible cause (formerly aclaudit B12). Refusing it at load turns that
// into a message naming the grant and its replacement.
//
// `*` is not refused: it ranges over types and grants each one's implicit
// face, so it names no faced type (decision D4). When a role holds `*` beside
// a refused entry, the message says so, because an operator deleting the bare
// entry may otherwise believe the wildcard covers the faces.
//
// An alias of a faced type is refused too, bare or with a face. Grant
// matching compares type names literally and does not resolve aliases, so
// `alias@face` would load clean and grant nothing. The message names the
// canonical form.
//
// Read grants are not checked: a bare read grant covers every face.
//
// Types in refused were already reported by
// Policy.validateIdentityStructure; the fix there is to remove their faces,
// so suggesting face-named grants for them would contradict it.
//
// Every offending entry is reported, not only the first.
func (p *Policy) validateFacedWriteGrants(meta MetamodelView, refused map[string]bool) []error {
	var errs []error
	for _, name := range sortedRoleNames(p.Roles) {
		role := p.Roles[name]
		for _, verb := range []struct {
			name string
			list []string
		}{
			{"create", role.Create}, {"update", role.Update}, {"delete", role.Delete},
		} {
			errs = append(errs, facedWriteGrantErrors(meta, refused, name, verb.name, verb.list)...)
		}
		errs = append(errs, renameAliasErrors(meta, name, role.Rename)...)
	}
	return errs
}

// renameAliasErrors refuses a `rename:` entry that names a faced type through
// an alias. Grant matching compares type names literally, so the entry would
// load clean and grant nothing. A face-named entry was already refused by
// Policy.Validate.
func renameAliasErrors(meta MetamodelView, role string, list []string) []error {
	var errs []error
	for _, entry := range list {
		if entry == "*" || isStateGrant(entry) {
			continue
		}
		canonical, faces := meta.FaceNames(entry)
		if len(faces) == 0 || canonical == entry {
			continue
		}
		errs = append(errs, fmt.Errorf("acl: roles.%s.rename: %q names the type through the alias %q; "+
			"a rename grant must use the canonical type name: %q", role, entry, entry, canonical))
	}
	return errs
}

// facedWriteGrantErrors checks one role's grant list for one verb.
func facedWriteGrantErrors(meta MetamodelView, refused map[string]bool, role, verb string, list []string) []error {
	var errs []error
	seen := map[string]bool{}
	hasWildcard := slices.Contains(list, "*")
	for _, entry := range list {
		if entry == "*" || seen[entry] {
			continue
		}
		seen[entry] = true
		typeName, face, faced := strings.Cut(entry, entity.StateRefSeparator)
		canonical, faces := meta.FaceNames(typeName)
		if len(faces) == 0 || refused[canonical] {
			continue
		}
		var msg string
		switch {
		case typeName != canonical && faced:
			msg = fmt.Sprintf("acl: roles.%s.%s: %q names the type through the alias %q; "+
				"a write grant must use the canonical type name: %q",
				role, verb, entry, typeName, canonical+entity.StateRefSeparator+face)
		case faced:
			continue
		default:
			msg = fmt.Sprintf("acl: roles.%s.%s: %q names a type that declares faces; "+
				"a write grant must name the face: %s",
				role, verb, entry, faceSuggestions(canonical, faces))
			if hasWildcard && !namesAnyFace(list, canonical) {
				msg += fmt.Sprintf(" (note: %q in this list reaches only types without faces, "+
					"so it does not cover %q)", "*", canonical)
			}
		}
		errs = append(errs, errors.New(msg))
	}
	return errs
}

func faceSuggestions(canonical string, faces []string) string {
	suggestions := make([]string, len(faces))
	for i, f := range faces {
		suggestions[i] = fmt.Sprintf("%q", canonical+entity.StateRefSeparator+f)
	}
	return strings.Join(suggestions, ", ")
}

// namesAnyFace reports whether list holds a `type@face` entry for canonical.
// Only the canonical name counts: an alias entry is refused on its own and
// grants nothing.
func namesAnyFace(list []string, canonical string) bool {
	for _, entry := range list {
		if isStateGrant(entry) && grantTypeOf(entry) == canonical {
			return true
		}
	}
	return false
}

// validateIdentityStructure refuses faces and content scope on the parts of
// the schema the ACL resolves identities and roles through (TKT-7IZHP0 D7,
// A16).
//
// A principal must resolve to one identity, and role conferral walks
// tail-less edges between entity ids. A faced user type would let two faces of
// one id carry different identity values; a faced member or group type would
// let a face nobody published confer a role; a content-scoped edge attaches to
// one face and has no identity-level meaning. Each is refused at load.
//
// Checked:
//
//   - user_entity_type must be faceless.
//   - The effective membership relation must be identity-scoped, and every
//     type it connects (members and groups) must be faceless.
//   - Each role_relations type with `confers:` must be identity-scoped, and its source types
//     (the role holders: users and groups) must be faceless. Its target types
//     may declare faces: the conferred role applies to the whole entity, and
//     face-named write grants then decide which faces it may write.
//   - Each inherit_roles_through type must be identity-scoped.
//
// A relation the schema does not declare is skipped: it can hold no edges,
// and `member-of` is walked by default whether or not it is declared.
//
// Role holders are refused conservatively: every declared source type of a
// conferring relation must be faceless, even one that holds no edges today.
//
// The second result holds the canonical names of the types refused here, so
// Policy.validateFacedWriteGrants does not report them a second time with a
// contradicting fix.
func (p *Policy) validateIdentityStructure(meta MetamodelView) (errs []error, refused map[string]bool) {
	refused = map[string]bool{}
	if userType := strings.TrimSpace(p.UserEntityType); userType != "" {
		if canonical, faces := meta.FaceNames(userType); len(faces) > 0 {
			refused[canonical] = true
			errs = append(errs, fmt.Errorf(
				"acl: user_entity_type %q declares faces (%s); a principal must resolve to one "+
					"identity, so the user type must be faceless: remove its `faces:` block or name "+
					"a faceless type", userType, strings.Join(faces, ", ")))
		}
	}

	membership := p.EffectiveMembershipRelation()
	if info := meta.RelationInfo(membership); info.Exists {
		key := fmt.Sprintf("membership_relation %q", membership)
		if p.MembershipRelation == "" {
			key += fmt.Sprintf(" (%q is the default membership relation; set `membership_relation:` "+
				"if this is not your membership relation)", membership)
		}
		errs = append(errs, contentScopeError(key, membership, info)...)
		errs = append(errs, facedEndpointErrors(meta, refused, key, "member", info.From)...)
		errs = append(errs, facedEndpointErrors(meta, refused, key, "group", info.To)...)
	}

	for _, relType := range sortedKeys(p.RoleRelations) {
		info := meta.RelationInfo(relType)
		if !info.Exists || relType == membership || p.RoleRelations[relType].Confers == "" {
			// Undeclared, already checked above more strictly as the membership
			// relation, or a write gate only (no `confers:`), which the
			// resolver never walks.
			continue
		}
		key := "role_relations." + relType
		errs = append(errs, contentScopeError(key, relType, info)...)
		errs = append(errs, facedEndpointErrors(meta, refused, key, "role holder", info.From)...)
	}

	for _, relType := range p.InheritRolesThrough {
		if info := meta.RelationInfo(relType); info.Exists {
			errs = append(errs, contentScopeError("inherit_roles_through "+fmt.Sprintf("%q", relType),
				relType, info)...)
		}
	}
	return errs, refused
}

func contentScopeError(key, relType string, info RelationInfo) []error {
	if !info.Content {
		return nil
	}
	return []error{fmt.Errorf(
		"acl: %s: relation %q is declared `scope: content`; a relation the ACL walks for "+
			"roles must be identity-scoped: remove `scope: content` from %q in schema.yaml",
		key, relType, relType)}
}

// facedEndpointErrors reports each faced type among types, once per canonical
// type, and adds it to refused. side names the role the type plays ("member",
// "group", "role holder").
func facedEndpointErrors(meta MetamodelView, refused map[string]bool, key, side string, types []string) []error {
	var errs []error
	seen := map[string]bool{}
	for _, t := range types {
		canonical, faces := meta.FaceNames(t)
		if len(faces) == 0 || seen[canonical] {
			continue
		}
		seen[canonical] = true
		refused[canonical] = true
		errs = append(errs, fmt.Errorf(
			"acl: %s: %s type %q declares faces (%s); ACL users, groups and members must be "+
				"faceless so a principal resolves to one identity: remove its `faces:` block, or "+
				"use a relation between faceless types",
			key, side, canonical, strings.Join(faces, ", ")))
	}
	return errs
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
