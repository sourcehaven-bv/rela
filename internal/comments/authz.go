package comments

import "context"

// PermissionGate answers whether the ctx principal holds a named permission for
// one entity.
//
// Declared here, at the consumer, rather than beside the ACL: this package
// needs exactly one question answered and has no business seeing the rest of
// the resolver. The wiring site supplies an adapter over
// acl.Request.HoldsPermissionForEntity, which resolves permissions conferred by
// an ownership relation to the subject as well as global grants.
//
// Nil: a nil PermissionGate means "no policy configured"; [Authorizer] treats
// that as permitting everything, matching the CLI/desktop tier where there is
// no principal to authorize.
type PermissionGate interface {
	HoldsPermission(ctx context.Context, subjectID, permission string) bool
}

// TargetReadGate answers whether the ctx principal may read the target entity
// at all.
//
// Separate from [PermissionGate] because it answers a different question and
// has a different failure mode: a denied read must be indistinguishable from a
// missing entity, whereas a denied permission may say which permission was
// missing (a permission name is config, and config is not secret).
//
// Nil: a nil TargetReadGate means "no policy configured" and permits.
type TargetReadGate interface {
	CanRead(ctx context.Context, entityType, entityID string) bool
}

// Action is a thing a principal may want to do to a comment.
type Action int

const (
	// ActionRead lists an entity's comments.
	ActionRead Action = iota
	// ActionAdd adds a comment.
	ActionAdd
	// ActionUpdate edits or resolves an existing comment.
	ActionUpdate
	// ActionDelete removes an existing comment.
	ActionDelete
)

// Authorizer decides whether a principal may act on a target's comments.
//
// It exists so the permission vocabulary is interpreted in ONE place: the
// own/any split, the read floor and the inert/fail-closed rule are decided
// here rather than at each HTTP handler, where a missed check is invisible.
type Authorizer struct {
	perms  PermissionGate
	reads  TargetReadGate
	active bool
}

// NewAuthorizer returns an Authorizer over the supplied gates.
//
// policyActive distinguishes the two ways a gate can be absent, and the
// distinction is load-bearing (the same rule the statemachine transition guard
// documents):
//
//   - No policy configured at all: the Authorizer is INERT and permits. This is
//     the CLI/desktop tier, where there is no principal to authorize.
//   - A policy IS configured but a gate is missing: that is a wiring failure on
//     a served path, and the Authorizer FAILS CLOSED. A policy-backed
//     deployment must not silently open commenting because plumbing broke.
func NewAuthorizer(perms PermissionGate, reads TargetReadGate, policyActive bool) *Authorizer {
	return &Authorizer{perms: perms, reads: reads, active: policyActive}
}

// CanRead reports whether the principal may list target's comments.
//
// Two conditions, both required: the target must be readable, and the principal
// must hold `comment:read` for it. The read floor is what stops a comment
// thread becoming an existence oracle — without it, a principal granted
// comment:read globally could probe which entities exist by asking for their
// comments.
func (a *Authorizer) CanRead(ctx context.Context, target Target) bool {
	if !a.active {
		return true
	}
	return a.canReadTarget(ctx, target) && a.holds(ctx, target.ID, permRead)
}

// CanAdd reports whether the principal may add a comment to target.
//
// Note this does NOT require `comment:read`: write-only commenting is a
// deliberate posture (leave a remark on something you cannot otherwise
// discuss), mirroring the entity-verb rule where create is exempt from the
// covering-read requirement. The target must still be readable — you cannot
// comment on an entity you cannot see.
func (a *Authorizer) CanAdd(ctx context.Context, target Target) bool {
	if !a.active {
		return true
	}
	return a.canReadTarget(ctx, target) && a.holds(ctx, target.ID, permAdd)
}

// CanUpdate reports whether the principal may edit or resolve c.
//
// The -any permission implies the -own one, so a moderator needs only
// comment:update-any rather than both.
func (a *Authorizer) CanUpdate(ctx context.Context, target Target, c Comment, principalUser string) bool {
	return a.canMutate(ctx, target, c, principalUser, permUpdateOwn, permUpdateAny)
}

// CanDelete reports whether the principal may remove c.
func (a *Authorizer) CanDelete(ctx context.Context, target Target, c Comment, principalUser string) bool {
	return a.canMutate(ctx, target, c, principalUser, permDeleteOwn, permDeleteAny)
}

// canMutate is the shared own/any decision.
//
// Ownership is a string comparison of the stored author against the request
// principal — NOT the graph's ownership mechanism, which tests for a conferring
// EDGE and therefore cannot see a comment at all. Keeping the comparison here
// means the ACL never has to learn about non-graph records.
func (a *Authorizer) canMutate(
	ctx context.Context, target Target, c Comment, principalUser, ownPerm, anyPerm string,
) bool {
	if !a.active {
		return true
	}
	if !a.canReadTarget(ctx, target) {
		return false
	}
	if a.holds(ctx, target.ID, anyPerm) {
		return true
	}
	// An empty author or principal can never establish ownership. Comments are
	// refused at write time when the author cannot be resolved, so this is
	// belt-and-braces against a hand-edited file.
	if principalUser == "" || c.Author == "" || c.Author != principalUser {
		return false
	}
	return a.holds(ctx, target.ID, ownPerm)
}

// canReadTarget applies the read floor.
func (a *Authorizer) canReadTarget(ctx context.Context, target Target) bool {
	if a.reads == nil {
		// Policy is active but the read gate is missing: fail closed.
		return false
	}
	return a.reads.CanRead(ctx, target.Type, target.ID)
}

// holds resolves one named permission for the target entity.
func (a *Authorizer) holds(ctx context.Context, subjectID, perm string) bool {
	if a.perms == nil {
		// Policy is active but the permission gate is missing: fail closed.
		return false
	}
	return a.perms.HoldsPermission(ctx, subjectID, perm)
}

// Permission names, duplicated from internal/acl as plain strings.
//
// This package must not import internal/acl: comments sit below the
// authorization layer and take a narrow gate interface instead, so importing
// the resolver would invert that. The strings are part of the operator-facing
// config vocabulary and so are stable by contract; acl's constants remain the
// single source consumers reference.
const (
	permRead      = "comment:read"
	permAdd       = "comment:add"
	permUpdateOwn = "comment:update-own"
	permUpdateAny = "comment:update-any"
	permDeleteOwn = "comment:delete-own"
	permDeleteAny = "comment:delete-any"
)
