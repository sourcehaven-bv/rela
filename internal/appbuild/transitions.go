package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
