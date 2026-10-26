package acl

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// JobGrants answers, from acl.yaml alone, what a job identity may write. A
// job identity is a `system:` or `integration:` name an operator configures
// as the identity of background work, such as a realtime session's
// `run_as`. Build it with [JobIdentityGrants].
//
// Its answers count only the roles the identity's assignment names and the
// `everyone` role. They leave out asserted roles, local roles and client
// ceilings, because a job identity carries no claims, has no user entity
// and has no principal type or scopes. For a person those are real, so a
// JobGrants answer about a person would be wrong: ask an [*Request] instead.
//
// A `when:` on a field or option grant counts as granted: the answer is the
// most the policy can allow, and the write path asks again on the row.
type JobGrants struct {
	policy *Policy
	roles  []RoleDef
}

// JobIdentityGrants returns what p grants the job identity user through
// its assignment and the `everyone` role.
//
// Nil: p is accepted and grants nothing.
func JobIdentityGrants(p *Policy, user string) JobGrants {
	g := JobGrants{policy: p}
	if p == nil {
		return g
	}
	if name, ok := p.Assignments[user]; ok {
		if role, defined := p.Roles[name]; defined {
			g.roles = append(g.roles, role)
		}
	}
	if name := EveryoneRole; name != p.Assignments[user] {
		if role, defined := p.Roles[name]; defined {
			g.roles = append(g.roles, role)
		}
	}
	return g
}

// AnyRoleGrants reports whether some role of p (everyone included) grants
// op on typ@face and, for a non-empty field, field write by the union rule
// of [JobGrants.FieldWrite]. It is a policy-wide floor for load warnings,
// not an authorization answer: it ignores assignments, asserted roles,
// local roles and client ceilings. Only [OpCreate] and [OpUpdate] are
// answered.
//
// Nil: p is accepted and grants nothing.
func AnyRoleGrants(p *Policy, op Op, typ string, face entity.Face, field string) bool {
	if p == nil {
		return false
	}
	g := JobGrants{policy: p}
	for _, r := range p.Roles {
		g.roles = append(g.roles, r)
	}
	if !g.Verb(op, typ, face) {
		return false
	}
	return field == "" || g.FieldWrite(op, typ, face, field)
}

// Verb reports whether a role grants op on the face of typ. Only
// [OpCreate] and [OpUpdate] are answered; any other op grants nothing. The
// zero face is the default state, which an unfaced type addresses.
func (g JobGrants) Verb(op Op, typ string, face entity.Face) bool {
	if op != OpCreate && op != OpUpdate {
		return false
	}
	return slices.ContainsFunc(g.roles, func(r RoleDef) bool { return GrantsVerbOnState(r, op, typ, face) })
}

// FieldWrite reports whether the field grants let op write field on the
// face of typ. It folds the roles as the affordance resolver does
// (BUG-Y5UF3K): a `fields:` block of any role closes the type, and the
// field is then writable when some block grants it, or when some role
// without a block holds op on the face. A `visible:` block closes
// visibility the same way, with read standing in for op, because a write
// to a field the writer cannot see is refused too.
func (g JobGrants) FieldWrite(op Op, typ string, face entity.Face, field string) bool {
	writable := g.fold(typ, face, field, Verb(op), func(r RoleDef, key string) ([]FieldGrant, bool) {
		b, ok := r.Fields[key]
		return b, ok
	})
	visible := g.fold(typ, face, field, VerbRead, func(r RoleDef, key string) ([]FieldGrant, bool) {
		b, ok := r.Visible[key]
		return b, ok
	})
	return writable && visible
}

// fold is one field dimension of [JobGrants.FieldWrite]: blocks returns a
// role's block under a key, and verb is what a role without a block must
// hold to open every field.
func (g JobGrants) fold(
	typ string, face entity.Face, field string, verb Verb, blocks func(RoleDef, string) ([]FieldGrant, bool),
) bool {
	closed, open := false, false
	for _, r := range g.roles {
		declared := false
		for _, key := range blockKeys(typ, face) {
			b, ok := blocks(r, key)
			if !ok {
				continue
			}
			declared = true
			if slices.ContainsFunc(b, func(fg FieldGrant) bool { return fg.Field == field }) {
				open = true
			}
		}
		closed = closed || declared
		if !declared && GrantsVerbOnFace(r, verb, typ, face) {
			open = true
		}
	}
	return !closed || open
}

// Option reports whether the option grants let op write value into the
// enum field on the face of typ. An `options:` grant naming the field
// closes it; the value is then allowed when some role grants it, or when
// some role without an `options:` block for the type holds op on the face.
func (g JobGrants) Option(op Op, typ string, face entity.Face, field, value string) bool {
	closed, open := false, false
	for _, r := range g.roles {
		declared := false
		for _, key := range blockKeys(typ, face) {
			b, ok := r.Options[key]
			if !ok {
				continue
			}
			declared = true
			for _, og := range b {
				if og.Field != field {
					continue
				}
				closed = true
				if og.Option == value {
					open = true
				}
			}
		}
		if !declared && GrantsVerbOnState(r, op, typ, face) {
			open = true
		}
	}
	return !closed || open
}

// Relates reports whether a role lets the identity create a relation of
// relType from an entity of fromType, decided as [Request.AuthorizeWrite]
// decides a relation create: the delegate permission of a role-conferring
// relation first, then `relation_grants:` (which needs update on a named
// tail), then create on the source type. tail is the edge's face: the
// source's face for a `scope: content` edge, else zero. familyFaces are the
// faces fromType declares when tail is zero; each must allow the create.
func (g JobGrants) Relates(relType, fromType string, tail entity.Face, familyFaces []entity.Face) bool {
	if g.policy == nil {
		return false
	}
	if rr, ok := g.policy.RoleRelations[relType]; ok && rr.RequiresPermission != "" &&
		!g.holdsPermission(rr.RequiresPermission) {

		return false
	}
	if perm, ok := g.policy.relationPermissionFor(relType, OpCreate); ok && g.holdsPermission(perm) &&
		(tail.IsImplicit() || g.Verb(OpUpdate, fromType, tail)) {

		return true
	}
	faces := familyFaces
	if len(faces) == 0 {
		faces = []entity.Face{tail}
	}
	for _, f := range faces {
		if !g.Verb(OpCreate, fromType, f) {
			return false
		}
	}
	return true
}

// holdsPermission reports whether a role grants perm.
func (g JobGrants) holdsPermission(perm string) bool {
	return slices.ContainsFunc(g.roles, func(r RoleDef) bool { return slices.Contains(r.Permissions, perm) })
}

// blockKeys are the keys of a field block that apply to a row of typ at
// face: the bare type, and the `type@face` key for a named face.
func blockKeys(typ string, face entity.Face) []string {
	if face.IsImplicit() {
		return []string{typ}
	}
	return []string{typ, typ + entity.StateRefSeparator + string(face)}
}
