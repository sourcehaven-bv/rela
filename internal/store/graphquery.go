package store

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// GraphQuery describes a graph-shape question: "entities of EntityType
// where they have a matching inbound or outbound relation." The DSL is
// intentionally generic — no ACL or other consumer vocabulary — so
// future consumers (ACL read filtering, analyze tools, search) can
// compose against one stable shape.
//
// All three backends ship a default implementation that delegates to
// [internal/store/graphquerynaive] (iterate-and-filter in Go). A
// future SQL-pushdown implementation in pgstore is tracked as a
// follow-up.
type GraphQuery struct {
	EntityType  string
	HasInbound  *RelationPredicate // entity has matching relation FROM (expanded) endpoints
	HasOutbound *RelationPredicate // entity has matching relation TO (expanded) endpoints
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
	Endpoints []string
	OfTypes   []string

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
}
