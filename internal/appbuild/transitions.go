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
// Served-vs-inert lives here: the enforcer's guard question is only meaningful
// when there is an authenticated principal with a policy. On a direct CLI/no-
// policy write there is no [acl.Request] on the context (NopACL never puts one
// there), so the guard is inert — it allows, matching the "no principal → no
// authorization to enforce" tier rule. A misconfigured served path that somehow
// lacks a Request also allows here; the top-level entity-write ACL gate has
// already run, so this is not the only line of defense.
type transitionGuard struct{}

func (transitionGuard) HoldsPermission(ctx context.Context, subjectID, permission string) bool {
	req := acl.FromContext(ctx)
	if req == nil {
		return true // no policy/principal in scope → guard inert (direct/CLI path)
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
func CompileTransitions(meta *metamodel.Metamodel, st store.Store) (TransitionWiring, error) {
	set, err := statemachine.Compile(meta)
	if err != nil {
		return TransitionWiring{}, err
	}
	return TransitionWiring{Enforcer: set, Guard: transitionGuard{}, Graph: transitionGraph{st: st}}, nil
}
