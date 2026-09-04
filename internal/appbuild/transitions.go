package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// transitionGuard adapts the ACL to the [statemachine.Guard] the transition
// enforcer needs. It answers "does the ctx principal hold `permission` for the
// subject entity" using the SUBJECT-AWARE resolver
// ([acl.Request.HoldsPermissionForEntity]) — so a permission conferred by an
// ownership relation to the subject is honored, not just global grants.
//
// Served-vs-inert lives here, and the distinction is deliberate (RR-UOBUC):
//
//   - No policy configured (NopACL / no acl.yaml): policyActive is false. The
//     guard is inert — it allows — matching the "no principal → no authorization
//     to enforce" tier rule (direct CLI writes, demos).
//   - A policy IS configured (Declarative) but no [acl.Request] is on the
//     context: this is a misconfiguration (a served path that lost its
//     Request-attach middleware, or a background job reusing a bare ctx). The
//     guard fails CLOSED — it denies — because a policy-backed deployment must
//     not silently open a governed transition just because plumbing broke. This
//     matches the rest of the ACL's fail-closed posture (the router 500s on an
//     unresolved principal; role walks abort rather than over-grant).
type transitionGuard struct{ policyActive bool }

func (g transitionGuard) HoldsPermission(ctx context.Context, subjectID, permission string) bool {
	req := acl.FromContext(ctx)
	if req == nil {
		// No Request: inert when there is no policy at all; fail closed when a
		// policy exists but the Request is unexpectedly absent.
		return !g.policyActive
	}
	return req.HoldsPermissionForEntity(ctx, subjectID, permission)
}

// transitionGraph adapts the store to the [statemachine.GraphLookup] a `when:`
// precondition needs (has_relation / count_relations). One outgoing scan per
// evaluation answers both host functions.
type transitionGraph struct{ st store.Store }

func (g transitionGraph) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for rel, err := range g.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil {
			return counts
		}
		counts[rel.Type]++
	}
	return counts
}

// TransitionWiring bundles the compiled state machines and the collaborators
// the entitymanager needs to enforce them. Returned by [CompileTransitions] so
// every wiring site (production assemble + test fixtures) builds the enforcer
// the same way.
type TransitionWiring struct {
	Enforcer *statemachine.Set
	Guard    statemachine.Guard
	Graph    statemachine.GraphLookup
}

// CompileTransitions builds the executable state machines from the metamodel
// and the wiring collaborators the entitymanager needs. A metamodel with no
// transitions yields an empty (no-op) enforcer. Returns an error only when a
// machine is malformed (surfaced at boot). Exported so test fixtures wire the
// enforcer through the same path as production.
//
// resolvedACL determines the guard's fail-closed posture: when it is a
// policy-backed [*acl.Declarative], a served write that is missing its
// [acl.Request] is denied rather than allowed (RR-UOBUC). With NopACL /
// ReadOnlyACL there is no policy, so the guard stays inert.
func CompileTransitions(meta *metamodel.Metamodel, st store.Store, resolvedACL acl.ACL) (TransitionWiring, error) {
	set, err := statemachine.Compile(meta)
	if err != nil {
		return TransitionWiring{}, err
	}
	_, policyActive := resolvedACL.(*acl.Declarative)
	return TransitionWiring{
		Enforcer: set,
		Guard:    transitionGuard{policyActive: policyActive},
		Graph:    transitionGraph{st: st},
	}, nil
}

// copyReadGate answers "may this principal READ the copy's source", the first
// of authorizeCopy's three checks (TKT-WRLDAPI item 5).
//
// # Same shape and same tiering as [transitionGuard], deliberately
//
// It resolves the [acl.Request] from the context per call, because the gate is
// per-PRINCIPAL while Deps is built once at startup. The three tiers match:
//
//   - No policy at all: inert, permits. Every other read on such a deployment
//     is raw too (CLI, demos), so gating this one would be theater.
//   - A policy IS configured but no Request is on the context: FAIL CLOSED.
//     That is a served path that lost its Request-attach middleware, or a
//     background job on a bare ctx — a policy-backed deployment must not open
//     a governed read because plumbing broke.
//
// # Why this had to be wired at all
//
// Before item 5 nothing in appbuild set any Copy* dep, so entitymanager's
// `CopyReadGate` was nil in EVERY deployment — while its own godoc says "what
// it must never be is absent on a deployment that HAS a policy". A copy's
// source read then took the no-policy branch, so an unguarded cross-entity
// copy read with no row gate and no redaction.
type copyReadGate struct{ policyActive bool }

func (g copyReadGate) PermitsReadFace(
	ctx context.Context, entityType, entityID string, face entity.Face,
) (bool, error) {
	req := acl.FromContext(ctx)
	if req == nil {
		return !g.policyActive, nil
	}
	return req.PermitsReadFace(ctx, entityType, entityID, face)
}

// copyVisibility is the CALLER'S read gate for CROSS-ENTITY copies: it returns
// the source as that principal may see it, so a redacted field cannot travel
// into an entity with a different audience (design doc §9.2).
//
// Same-entity copies deliberately do NOT use this — they run elevated, because
// hidden fields belong to the same entity under the same policy, and routing
// them through a redacting read would destroy the fields the principal cannot
// see (the read-modify-write bug the codebase forbids everywhere).
//
// Nil-Request tiering matches [copyReadGate]: inert without a policy, and with
// a policy present but no Request it reports NOT FOUND rather than handing back
// an ungated entity — the fail-closed direction for a read.
//
// It is the caller's gate in all three respects a read gate has: the ROW
// (PermitsRead), the FACE (a `type@face` grant that excludes the source face
// is a miss), and the FIELDS (`visible:` redaction through the same
// [visibility.FieldRedactor] the API's read path uses). An earlier cut did
// only the row and read the default face raw, so a cross-entity copy from a
// non-bare face read the wrong face, and `visible:`-hidden properties
// traveled into the new entity — the exact bypass this form exists to
// prevent, and one the unit tests could not see because they injected a
// redacting fake.
type copyVisibility struct {
	st           store.Store
	redact       visibility.FieldRedactor
	policyActive bool
}

func (v copyVisibility) Get(
	ctx context.Context, entityType, id string, face entity.Face,
) (*entity.Entity, bool, error) {
	req := acl.FromContext(ctx)
	if req == nil {
		if v.policyActive {
			// Fail closed: absent rather than ungated. Indistinguishable from
			// a genuine miss, which is what every read gate here promises.
			return nil, false, nil
		}
		e, err := v.st.GetEntityState(ctx, id, face)
		if err != nil {
			return nil, false, nil //nolint:nilerr // a miss is not-found, matching the gated path
		}
		return e, true, nil
	}
	permitted, err := req.PermitsReadFace(ctx, entityType, id, face)
	if err != nil {
		return nil, false, err
	}
	if !permitted {
		return nil, false, nil
	}
	e, err := v.st.GetEntityState(ctx, id, face)
	if err != nil {
		return nil, false, nil //nolint:nilerr // a miss is not-found
	}
	if hidden := v.redact.HiddenProperties(ctx, e); len(hidden) > 0 {
		e = e.Clone()
		for name := range hidden {
			delete(e.Properties, name)
		}
	}
	return e, true, nil
}
