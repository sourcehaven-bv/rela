package acl

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Declarative is the policy-driven [ACL] implementation. It composes
// a [Policy] (loaded from `acl.yaml`) with the [principal.Principal]
// carried on each request's context to answer [ACL.AuthorizeWrite].
//
// Every [WriteRequest] carries a [Subject] (sum of [EntitySubject] /
// [RelationSubject]) — the resolver dispatches on the variant and
// runs the full path: group expansion via member-of, containment
// inheritance through `inherit_roles_through`, and typed-Source
// attribution for the audit log.
//
// Semantics:
//
//   - **Union semantics.** Any role granting write on the target
//     entity type → allow. The returned RuleID names the first role
//     that matched, for debuggability.
//   - **Delegate-X tamper resistance.** Writes to a relation type
//     listed in [Policy.RoleRelations] require the writer to hold the
//     declared permission. See [RoleRelationDef.RequiresPermission].
//   - **Unstamped principals are hard-denied.** A principal with
//     User="" / User="unknown" or Tool="" / Tool="unknown" fails the
//     [ForPrincipal] check; the deny surfaces as RuleKind="role-grant"
//     with a Reason that names ErrUnstampedPrincipal.
type Declarative struct {
	policy       *Policy
	graph        Graph              // required: NewDeclarative rejects nil
	graphQueryer store.GraphQueryer // required: needed by Request.Visible
}

// NewDeclarative wraps a [Policy] + [Graph] + [store.GraphQueryer] as
// an [ACL]. All three must be non-nil:
//
//   - Policy is the static role / assignment definitions.
//   - Graph supplies the read-side access the resolver needs for
//     member-of walks and ancestor probes used by AuthorizeWrite.
//   - GraphQueryer supplies [store.GraphQuery] / [store.GraphCount]
//     execution used by [Request.Visible] for per-entity read gating.
//
// Tests that don't exercise group expansion can pass [NullGraph];
// tests that don't exercise Visible can pass [NullGraphQueryer]
// (returns DenyAll-shaped zero matches). Production wiring (appbuild)
// passes the store as both Graph (via [NewStoreGraph]) and as the
// GraphQueryer.
func NewDeclarative(p *Policy, g Graph, gq store.GraphQueryer) (*Declarative, error) {
	if p == nil {
		return nil, errors.New("acl: NewDeclarative: policy must be non-nil")
	}
	if g == nil {
		return nil, errors.New("acl: NewDeclarative: graph must be non-nil")
	}
	if gq == nil {
		return nil, errors.New("acl: NewDeclarative: graphQueryer must be non-nil")
	}
	return &Declarative{policy: p, graph: g, graphQueryer: gq}, nil
}

// Policy returns the policy this Declarative was constructed with.
// Exposed so downstream consumers (chiefly the affordance resolver)
// can read role/grant definitions from the same source the resolver
// uses for role attribution — eliminating the mismatched-pair foot-
// cannon where a caller might pass policyA to the affordance resolver
// while wiring a Declarative built from policyB (RR-WTLD).
//
// **The returned *Policy must be treated as immutable** (RR-9GN3).
// The resolver caches no policy state; every AuthorizeWrite reads
// fields back through this pointer. Mutating Roles, Assignments,
// or any nested map invalidates the resolver's safety guarantees
// from the next call onward, including the unstamped-principal
// check and the delegate-X gates. Callers that need a mutated
// policy build a fresh Declarative with [NewDeclarative].
func (d *Declarative) Policy() *Policy { return d.policy }

// AuthorizeWrite implements [ACL.AuthorizeWrite]. Opens a Request for
// the principal carried on ctx, then delegates to
// [Request.AuthorizeWrite] which dispatches on Subject variant. An
// unstamped principal short-circuits to a structured deny.
func (d *Declarative) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
	r, err := d.ForPrincipal(principal.From(ctx))
	if err != nil {
		return Decision{
			Allow:    false,
			RuleKind: "role-grant",
			RuleID:   "-",
			Reason:   fmt.Sprintf("acl.ForPrincipal: %v", err),
		}
	}
	return r.AuthorizeWrite(ctx, req)
}

// roleGrantsWrite reports whether `role.Write` covers `target` —
// either by an exact match or by the wildcard `"*"`.
func roleGrantsWrite(role RoleDef, target string) bool {
	for _, w := range role.Write {
		if w == "*" || w == target {
			return true
		}
	}
	return false
}
