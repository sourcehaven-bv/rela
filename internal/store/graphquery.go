package store

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// GraphQuery describes a graph-shape question: "entities of EntityType
// whose own properties match, and which have (or lack) a matching
// inbound or outbound relation." The DSL is intentionally generic — no
// ACL or other consumer vocabulary — so consumers (ACL read filtering,
// analyze tools, search, next-action sources) can compose against one
// stable shape.
//
// All predicates are ANDed: an entity matches when every PropPredicate
// holds AND both relation predicates hold. A zero-value GraphQuery
// beyond EntityType matches every entity of that type.
//
// All three backends ship a default implementation that delegates to
// [internal/store/graphquerynaive] (iterate-and-filter in Go). A
// future SQL-pushdown implementation in pgstore is tracked as a
// follow-up.
type GraphQuery struct {
	EntityType  string
	Props       []PropPredicate    // entity's own properties match (AND)
	HasInbound  *RelationPredicate // entity has matching relation FROM (expanded) endpoints
	HasOutbound *RelationPredicate // entity has matching relation TO (expanded) endpoints
}

// PropOp is the comparison a [PropPredicate] applies. Deliberately only
// equality and its negation: ordered comparison (`due < 2026-01-01`)
// needs the property's declared type from the metamodel to avoid
// comparing dates lexicographically, and the store layer does not
// consult the metamodel. Typed comparison stays above the store in
// [internal/filter].
type PropOp int

const (
	// PropEqual matches when the property equals Value. With an empty
	// Value it means "is empty" — see [PropPredicate].
	PropEqual PropOp = iota
	// PropNotEqual is the negation. With an empty Value it means "is not
	// empty".
	PropNotEqual
)

// PropPredicate restricts a GraphQuery to entities whose own property
// matches. Multiple predicates on one query are ANDed.
//
// Emptiness follows [internal/propmatch], the single authoritative
// definition shared with [internal/filter] — a missing key and a
// present-but-empty value are the SAME state, because YAML frontmatter
// parses a valueless key to nil and an operator asking "is this field
// filled in?" does not distinguish the two:
//
//	{Property: "status", Op: PropEqual, Value: "doing"}  // status=doing
//	{Property: "billing_email", Op: PropEqual}           // is empty
//	{Property: "billing_email", Op: PropNotEqual}        // is not empty
//
// Note that an EMPTY property does not satisfy a PropNotEqual against a
// non-empty Value: an entity with no status is not in the "status is
// something other than doing" population. Treating it as a match would
// silently widen every exclusion filter to include unset rows.
type PropPredicate struct {
	Property string
	Op       PropOp
	Value    string
	// Scalar restricts a non-empty equality predicate to a scalar string and
	// lets SQL backends emit an indexable ->> comparison. It is ignored for
	// empty values and other operators.
	Scalar bool
}

// RelationPredicate restricts which relations the surrounding
// GraphQuery is willing to match through.
//
// Two transitive expansions, independent and composable:
//
//   - InheritThrough (endpoint-side) transitively expands Endpoints via
//     these relation types up to Depth. Example: ACL group expansion
//     (InheritThrough = ["member-of"]).
//   - EntityInheritThrough (entity-side) transitively expands the
//     candidate entity via these relation types up to EntityDepth; the
//     match succeeds if any ancestor of the candidate (including itself)
//     has the inbound/outbound edge. Example: ACL containment
//     inheritance (EntityInheritThrough = ["belongs-to"]).
type RelationPredicate struct {
	// Endpoints restricts which entities on the far side of the relation
	// count as a match.
	//
	// An EMPTY (or nil) Endpoints means "ANY endpoint": the predicate is
	// then purely about the edge existing, which is what an absence
	// query needs ("has no implements edge at all", with Negate). Note
	// this is a WIDENING, not a narrowing — a caller deriving endpoints
	// from a principal or a lookup MUST guard against accidentally
	// passing an empty set, or the predicate silently stops constraining
	// (see internal/acl.readQuery, which fails closed for exactly this).
	//
	// InheritThrough is inert when Endpoints is empty: there is nothing
	// to expand from, so the endpoint closure is skipped entirely.
	Endpoints []string
	OfTypes   []string

	// Negate inverts the predicate: the entity matches when NO relation
	// satisfies it ("has no billing-contact edge"). This is a separate
	// flag rather than an overload of a nil *RelationPredicate, because
	// nil already means "do not constrain this direction" — the two are
	// different questions, and conflating them would silently turn every
	// unconstrained query into an absence query.
	//
	// Negation composes with both expansions: with EntityInheritThrough
	// set, a negated predicate matches only when NO ancestor of the
	// candidate (including itself) has the edge.
	Negate bool

	InheritThrough []string
	Depth          int

	EntityInheritThrough []string
	EntityDepth          int
}

// GraphQueryer is the read-side interface for graph-shape queries.
// Embedded into Store; surfaces independently so backend
// implementations can be written and tested without the full Store.
type GraphQueryer interface {
	// GraphQuery returns an iterator over entities matching q. The
	// iterator yields (*entity.Entity, nil) for each match; on error
	// the iterator yields (nil, err) and terminates.
	GraphQuery(ctx context.Context, q GraphQuery) iter.Seq2[*entity.Entity, error]

	// GraphCount returns (matched, total): the number of entities of
	// q.EntityType that satisfy q's predicates, and the total number of
	// entities of q.EntityType ignoring those predicates. Callers use
	// (total - matched) for "filtered by" counts.
	GraphCount(ctx context.Context, q GraphQuery) (matched, total int, err error)

	// MatchingIDs answers: "of these candidate ids, which ones satisfy
	// q's predicates?" Returns a map keyed by every candidate id with
	// the boolean value indicating match (true) or no-match (false).
	// All input ids appear in the result regardless of outcome, so
	// callers can distinguish "absent because no-match" from "absent
	// because no answer."
	//
	// q is passed by value: implementations MUST NOT mutate it, and
	// the caller is free to reuse the input on the next call. ids is
	// the candidate set; an empty slice yields an empty map.
	//
	// Use this rather than threading id filters through GraphQuery —
	// it's the single-entity-visibility and batched-include shape used
	// by the ACL read gate.
	MatchingIDs(ctx context.Context, q GraphQuery, ids []string) (map[string]bool, error)
}
