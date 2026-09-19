package acl

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TraversalHop describes one hop of a caller-supplied relation traversal,
// before it is authorized. It is the UNGATED request — what the condition
// author asked for — which [Request.GateTraversal] turns into a
// [store.RelationPredicate] the principal is actually allowed to run.
//
// Callers build these from a compiled condition; they must never hand a
// store predicate straight to a query (see the security note on
// [store.RelationPredicate.EndpointMatch]).
type TraversalHop struct {
	// RelationTypes restricts which edges the hop follows. Empty follows
	// any type, matching [store.RelationPredicate.OfTypes].
	RelationTypes []string

	// EntityType is the traversed-TO type. REQUIRED: the gate resolves a
	// read query for it, and there is no read query for "some type". This
	// is the same ascription that resolves a union-targeted relation, so a
	// caller that already needs it for typing has it to hand.
	EntityType string

	// Props are the endpoint property filters the author wrote.
	Props []store.PropPredicate

	// Next chains one hop further. Nil ends the chain.
	Next *TraversalHop
}

// ErrTraversalDenied reports that a traversal cannot be authorized: the
// principal may not read the traversed-to type at all, so no rows could
// match. Callers surface it as an empty result, never as an error naming
// the entity type's contents.
var ErrTraversalDenied = errors.New("acl: traversal denied")

// GateTraversal compiles an untrusted traversal request into a store
// predicate the principal is permitted to evaluate.
//
// # Why this exists
//
// A traversal filters on properties of entities the query does NOT return.
// Nothing about the neighbor is ever serialized, so response redaction never
// fires — the leak is the CORRELATION between the filter and the result set,
// and it is binary-searchable: repeated queries narrow a hidden value
// arbitrarily. Two secrets are at stake, both named in docs/acl-security.md:
// a hidden entity's EXISTENCE, and a redacted field's VALUE.
//
// Three rules, all required, none sufficient alone:
//
//  1. ROW GATE. Each hop's read query for its entity type is ANDed into that
//     hop, so a traversal can only match neighbors the principal may read.
//     DenyAll on any hop fails the whole traversal closed.
//  2. FIELD GATE. A property whose `visible:` grant is CONDITIONAL is not
//     filterable, at all, by anyone. The row gate does not help here: the
//     principal can see the row and still not be entitled to the value, and a
//     filter would reveal it. See [Policy.ConditionallyVisible].
//  3. NO INHERITANCE EXPANSION. The gate never emits InheritThrough or
//     EntityInheritThrough. Those are anchored on the query's own entity type
//     and re-anchoring them mid-traversal would change who inherits from whom.
//
// A denied read on ANY hop returns [ErrTraversalDenied] rather than a
// predicate matching nothing, so the caller decides how to present it —
// but both readings are safe, because the gate never widens.
func (r *Request) GateTraversal(
	ctx context.Context, meta FieldVisibility, hop TraversalHop,
) (*store.RelationPredicate, error) {
	if hop.EntityType == "" {
		// Without a type there is nothing to authorize against. Refusing is
		// the only safe reading: resolving "any type" would mean composing no
		// read query, i.e. an ungated traversal.
		return nil, errors.New("acl: traversal hop: entity type is required")
	}

	for _, p := range hop.Props {
		if meta != nil && meta.ConditionallyVisible(hop.EntityType, p.Property) {
			return nil, fmt.Errorf(
				"acl: traversal hop: property %q on %q has a conditional visibility grant "+
					"and cannot be filtered on", p.Property, hop.EntityType)
		}
	}

	rq := r.ReadQuery(ctx, hop.EntityType)
	if rq.DenyAll {
		return nil, ErrTraversalDenied
	}
	// A face-restricted read means only SOME content states of the endpoint
	// are readable, and [store.EndpointPredicate] has no face field — so the
	// predicate cannot express the restriction. Emitting it anyway would
	// traverse through states the principal may not read. Refuse instead:
	// this is the same fail-closed direction the Any branch takes below, and
	// it is why faces are checked before the query is folded rather than
	// after.
	if len(rq.Faces) > 0 {
		return nil, ErrTraversalDenied
	}

	match := &store.EndpointPredicate{
		EntityType: hop.EntityType,
		Props:      hop.Props,
	}

	// The read query for the type is a GraphQuery; fold the parts that
	// constrain WHICH ROWS are readable into this hop. AllowAll contributes
	// nothing, which is correct rather than an omission.
	if !rq.AllowAll && rq.Query != nil {
		match.Props = append(match.Props, rq.Query.Props...)
		if rq.Query.HasInbound != nil {
			// The ACL's own inbound predicate carries its role-relation
			// expansions, which are anchored on the traversed-to type — it is
			// the ACL's, not the caller's, so its expansions are correct here.
			match.HasInbound = rq.Query.HasInbound
		}
		if len(rq.Query.Any) > 0 {
			// A disjunctive authorization ceiling cannot be expressed inside a
			// single endpoint predicate without flattening it into an OR that
			// EndpointPredicate has no field for. Refusing is the fail-closed
			// reading; widening to "no constraint" would be the escalation.
			return nil, ErrTraversalDenied
		}
	}

	if hop.Next != nil {
		nested, err := r.GateTraversal(ctx, meta, *hop.Next)
		if err != nil {
			return nil, err
		}
		match.HasOutbound = nested
	}

	return &store.RelationPredicate{
		OfTypes:       hop.RelationTypes,
		EndpointMatch: match,
	}, nil
}

// FieldVisibility answers whether a property's read visibility is
// CONDITIONAL — granted by a `visible:` rule carrying a `when:`.
//
// Declared at the consumer (CLAUDE.md "interfaces at the call site") so the
// gate binds to the one question it asks rather than to a whole policy.
// Nil: accepted — a caller with no policy skips the field gate, which is
// correct only when no `visible:` grants exist at all.
type FieldVisibility interface {
	ConditionallyVisible(entityType, property string) bool
}

// ConditionallyVisible implements [FieldVisibility]: it reports whether ANY
// role's `visible:` grant for this type/property carries a `when:` clause.
//
// The question is deliberately POLICY-WIDE and principal-independent. A
// per-principal answer would make the set of filterable properties vary by
// role, so two principals would get different compile results for the same
// condition — and the one who could filter would be an oracle for the one who
// could not. Asking of the policy instead means the refusal is a property of
// the SCHEMA, reported identically to everyone, which is also what lets it be
// a load-time error rather than a request-time one.
func (p *Policy) ConditionallyVisible(entityType, property string) bool {
	if p == nil {
		return false
	}
	for _, role := range p.Roles {
		for _, grant := range role.Visible[entityType] {
			if grant.When != "" && grant.Field == property {
				return true
			}
		}
	}
	return false
}
