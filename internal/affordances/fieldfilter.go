package affordances

import (
	"context"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// FieldVisibleOnEveryRow reports whether the principal on ctx sees property
// on every row of entityType it can read, so a list may filter and sort on it
// without computing over a value the principal cannot see (BUG-WJSYQT).
//
// It reads the same compiled grants, role verbs and client ceiling as
// [PolicyResolver.FieldVerdicts]. That method takes the union of the roles
// that apply to a row (BUG-Y5UF3K): a role with a `visible:` block for the
// type shows what the block grants on the row, a role without one shows every
// field when it reads the row's type and face, and the client ceiling then
// removes fields. Adding a role never removes a field.
//
// The principal's global roles apply to every row, and local roles can only
// add to them. So the field is visible on every row when the ceiling keeps it
// and a global role's `visible:` block grants it without `when:`. Otherwise
// the answer follows the read gate, which admits rows one of two ways:
//
//   - When a global role reads the type, the gate admits exactly the faces
//     the global roles read; local roles add none. The field is visible on
//     every row when, on each such face, some global role that reads the face
//     declares no `visible:` block for the type.
//   - When none does, a row is admitted only through a `role_relations` entry
//     whose role reads the row's face, and that role applies to the row. The
//     field is visible on every row when every such role either declares no
//     block for the type or grants the field without `when:`. When no role
//     reads the type, no row is admitted and the field counts as visible.
//
// The answer over-approximates on purpose, in the refusing direction: a grant
// with any `when:` counts as hiding, even one that does not depend on the
// row, and in the second case every role a relation can confer counts,
// whether or not the principal is linked to a row through it. Asking the
// graph which local roles the principal holds would make the refusal depend
// on rows and so turn it into an oracle. As it is, the answer depends only on
// the policy and the principal's own roles.
//
// A property the type does not declare is always visible: field verdicts deny
// only from the declared fields, so redaction never hides one.
func (r *PolicyResolver) FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool {
	if r.policy == nil {
		return true
	}
	def, known := r.meta.Entities[entityType]
	if !known {
		return true
	}
	if _, declared := def.Properties[property]; !declared {
		return true
	}
	globals, ceiling, verbs := r.globalRoleScope(ctx, entityType)
	if ceiling.Hides(property) {
		return false
	}
	if !r.typesWithVisible[entityType] {
		return true // no role declares a block, so no role hides a field
	}
	for _, role := range globals {
		if r.grants[grantKey{role, entityType}].showsUnconditionally(property) {
			return true
		}
	}
	faces := declaredFaces(def.Faces)
	readsType := func(role string) bool {
		return slices.ContainsFunc(faces, func(face entity.Face) bool {
			return verbs.RoleGrantsVerbOnFace(role, acl.VerbRead, entityType, face)
		})
	}
	blockless := func(role string) bool {
		g := r.grants[grantKey{role, entityType}]
		return g == nil || !g.declaredVisible
	}
	if slices.ContainsFunc(globals, readsType) {
		for _, face := range faces {
			readers := slices.DeleteFunc(slices.Clone(globals), func(role string) bool {
				return !verbs.RoleGrantsVerbOnFace(role, acl.VerbRead, entityType, face)
			})
			if len(readers) > 0 && !slices.ContainsFunc(readers, blockless) {
				return false
			}
		}
		return true
	}
	for _, rel := range r.policy.RoleRelations {
		role := rel.Confers
		if readsType(role) && !blockless(role) && !r.grants[grantKey{role, entityType}].showsUnconditionally(property) {
			return false
		}
	}
	// This includes the case where no role reads the type at all. The list
	// then has no rows, so the filter or sort has nothing to compare, and a
	// 200 with no rows says no more than the list endpoint always does.
	return true
}

// declaredFaces returns the faces a type's rows can be stored at, sorted: the
// declared ones, or the zero face for a type that declares none.
func declaredFaces[V any](faces map[string]V) []entity.Face {
	if len(faces) == 0 {
		return []entity.Face{""}
	}
	out := make([]entity.Face, 0, len(faces))
	for name := range faces {
		out = append(out, entity.Face(name))
	}
	slices.Sort(out)
	return out
}

// globalRoleScope resolves the principal's global role names, its client
// field ceiling for entityType, and the role verb check, the way
// [PolicyResolver.FieldVerdicts] does: from the request scope on ctx, or a
// fresh one when none is attached. An unstamped principal holds only the
// `everyone` role, when declared, and no ceiling can match it (see
// resolveViaDeclarative and applyClientCeiling).
func (r *PolicyResolver) globalRoleScope(
	ctx context.Context, entityType string,
) (roles []string, ceiling acl.FieldCeiling, verbs roleVerbGrants) {
	req := acl.FromContext(ctx)
	if req == nil {
		var err error
		req, err = r.declarative.ForPrincipal(principal.From(ctx))
		if err != nil {
			everyone, ok := r.policy.Roles[acl.EveryoneRole]
			if ok {
				roles = []string{acl.EveryoneRole}
			}
			return roles, acl.FieldCeiling{}, everyoneRoleGrants{everyone: everyone}
		}
	}
	for _, a := range req.Globals(ctx).Attributions {
		roles = append(roles, a.Role)
	}
	return roles, req.FieldCeilingFor(entityType), req
}

// showsUnconditionally reports whether these grants declare a `visible:`
// grant of field with no `when:`.
//
// Nil: accepted — no grants for the (role, type) show nothing.
func (g *compiledGrants) showsUnconditionally(field string) bool {
	if g == nil || !g.declaredVisible {
		return false
	}
	for _, fg := range g.visible {
		if fg.field == field && fg.program == nil {
			return true
		}
	}
	return false
}
