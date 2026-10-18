package acl

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// GrantsVerbOnFace reports whether role holds verb on the face of
// entityType, using the same predicates the row gate and the write gate use:
//
//   - [VerbRead]: [ReadFaces] covers the face. A bare type or "*" covers every
//     face; a `type@face` entry covers the face it names.
//   - [VerbCreate], [VerbUpdate] and [VerbDelete]: [GrantsVerbOnState], so "*"
//     covers only the unnamed face of a faceless type, exactly as on a write.
//
// It asks about one role definition and applies no client ceiling. Code that
// holds a principal asks [Request.RoleGrantsVerbOnFace] instead, which clamps
// the role and applies the ceiling first.
//
// An unknown verb grants nothing.
func GrantsVerbOnFace(role RoleDef, verb Verb, entityType string, face entity.Face) bool {
	if verb == VerbRead {
		faces, all := roleReadFaces(role, entityType)
		return all || slices.Contains(faces, face)
	}
	op, ok := verb.op()
	if !ok {
		return false
	}
	return GrantsVerbOnState(role, op, entityType, face)
}

// RoleGrantsVerbOnFace reports whether the named role, as this request's
// principal holds it, grants verb on the face of entityType.
//
// The client ceiling applies first and can only remove: a ceiling that denies
// the verb on the type answers false for every role, which also covers a
// role whose "*" survives clamping (see [filterTypes]). The role is then
// resolved through roleFor, so its lists are the clamped ones.
//
// roleName must be a role the principal holds, as attributed by
// [Request.ForEntity] or [Request.Globals]. This method does not check that:
// asked about a role the principal does not hold, it answers for that role
// anyway. An undeclared role grants nothing.
//
// The answer is memoised per (role, verb, type, face) for the life of the
// Request; the inputs are the immutable policy and the bound principal. It is
// safe to call from several goroutines.
func (r *Request) RoleGrantsVerbOnFace(roleName string, verb Verb, entityType string, face entity.Face) bool {
	question := roleVerbQuestion{role: roleName, verb: verb, entityType: entityType, face: face}
	r.roleVerbMu.Lock()
	held, seen := r.roleVerbAnswers[question]
	r.roleVerbMu.Unlock()
	if seen {
		return held
	}
	// Computed outside the lock: the answer is a pure function of immutable
	// inputs, so two goroutines racing on one question store the same value.
	held = r.roleGrantsVerbOnFace(question)
	r.roleVerbMu.Lock()
	if r.roleVerbAnswers == nil {
		r.roleVerbAnswers = map[roleVerbQuestion]bool{}
	}
	r.roleVerbAnswers[question] = held
	r.roleVerbMu.Unlock()
	return held
}

// roleVerbQuestion keys the memoised answers of [Request.RoleGrantsVerbOnFace].
type roleVerbQuestion struct {
	role       string
	verb       Verb
	entityType string
	face       entity.Face
}

// roleGrantsVerbOnFace computes one answer of [Request.RoleGrantsVerbOnFace].
func (r *Request) roleGrantsVerbOnFace(q roleVerbQuestion) bool {
	if !r.ceilingPermitsVerb(q.verb, q.entityType) {
		return false
	}
	role, ok := r.roleFor(q.role)
	if !ok {
		return false
	}
	return GrantsVerbOnFace(role, q.verb, q.entityType, q.face)
}

// ceilingPermitsVerb applies the client ceiling to a [Verb], routing read to
// the read axis and every write verb to its [Op].
func (r *Request) ceilingPermitsVerb(verb Verb, entityType string) bool {
	if verb == VerbRead {
		return r.ceiling.permitsRead(entityType)
	}
	op, ok := verb.op()
	if !ok {
		return false
	}
	return r.ceiling.permitsVerb(op, entityType)
}
