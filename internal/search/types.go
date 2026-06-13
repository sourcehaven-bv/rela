package search

import (
	"context"
	"errors"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Hit is a minimal result from a search operation.
type Hit struct {
	ID    string
	Type  string
	Title string
}

// Searcher provides search and filtering over entities. It is a top-level
// service separate from the Store: it builds its state by subscribing to
// store events or by wrapping a Backend. Smart backends (e.g. Postgres)
// can provide native implementations; simple backends use the generic
// implementation in this package.
type Searcher interface {
	Search(ctx context.Context, q Query) iter.Seq2[Hit, error]
}

// Backend is a pluggable full-text search index. It implements
// store.EntityObserver so it can be attached to a store as a write observer,
// and provides a Search method for querying. Implementations must be safe
// for concurrent use. Lifecycle (construction, population on startup,
// close) is the consumer's responsibility — the store does not manage it.
type Backend interface {
	store.EntityObserver

	// Search returns entity IDs matching the query text, ordered by relevance.
	// limit ≤ 0 means no limit.
	Search(text string, limit int) ([]string, error)
}

// WildcardType is the reserved scope-map key that supplies the default
// verdict for entity types without an explicit entry. It cannot collide
// with a real entity type: metamodel type names are identifiers.
const WildcardType = "*"

// TypeScope is one per-type visibility verdict inside a SearchVisible
// scope map. Exactly one meaning applies:
//
//   - AllowAll true → every entity of the type is visible.
//   - Query non-nil → only entities matching the GraphQuery are visible.
//     The query's EntityType must equal the scope-map key it is stored
//     under; the consumer constructing the scope owns that consistency.
//   - zero value → deny the type (an explicit deny entry is equivalent
//     to the type being absent from the map).
//
// The scope map is server-derived (from ACL policy verdicts), never
// wire-supplied.
type TypeScope struct {
	AllowAll bool
	Query    *store.GraphQuery
}

// ResolveTypeScope applies the scope lookup rule shared by every
// VisibleSearcher implementation: the exact type entry wins, else the
// reserved [WildcardType] entry, else deny (fail-closed). The second
// return reports whether any entry applied — false means the type is
// denied outright.
//
// Fail-closed is the load-bearing property: a nil or empty scope map
// denies everything, and an entity type the scope builder never saw
// (e.g. removed from the metamodel while its files remain on disk) is
// hidden rather than leaked.
func ResolveTypeScope(scope map[string]TypeScope, entityType string) (TypeScope, bool) {
	if ts, ok := scope[entityType]; ok {
		return ts, ts.AllowAll || ts.Query != nil
	}
	if ts, ok := scope[WildcardType]; ok {
		return ts, ts.AllowAll || ts.Query != nil
	}
	return TypeScope{}, false
}

// VisibleSearcher executes a search restricted to a per-type visibility
// scope. It is the read-side ACL seam for search: the consumer resolves
// ACL verdicts into the scope map (ACL stays at the call site — this
// package never sees a principal or a policy) and the implementation
// guarantees no hit outside the scope is ever yielded, on any backend.
//
// Contract, pinned by storetest.RunVisibleSearchTests (any new
// implementation must pass it):
//
//   - Scope lookup follows [ResolveTypeScope]: exact → "*" → deny.
//   - q.Limit bounds the number of VISIBLE hits — it is applied after
//     visibility filtering, never before. (A pre-visibility limit
//     starves restricted principals: the top-K candidates may all be
//     hidden while visible matches rank below them.)
//   - Relative order of visible hits equals the order the ungated
//     search on the same backend would yield them in.
//   - A [WildcardType] entry carrying a Query is invalid (a GraphQuery
//     targets one entity type) and yields an error.
//   - q.Sort is ignored, matching [Service.Search].
//
// Service plus this package's generic wrapper (NewVisible) serve the
// simple backends; smart backends (pgstore) implement it natively by
// composing visibility into the search query itself.
type VisibleSearcher interface {
	SearchVisible(ctx context.Context, q Query, scope map[string]TypeScope) iter.Seq2[Hit, error]
}

// ErrScope marks a SearchVisible failure that occurred while evaluating
// the visibility scope (GraphQuery/MatchingIDs execution), as opposed
// to a plain search-backend failure. Consumers route ErrScope failures
// through their ACL-error path. Implementations that cannot separate
// the two phases (pgstore runs one combined statement) wrap the whole
// failure in ErrScope — the query is the gate there.
var ErrScope = errors.New("search: visibility scope evaluation failed")

// Query describes a search request.
type Query struct {
	Text    string           // free-text search (ranked by relevance when set)
	Types   []string         // filter by entity types
	Filters []PropertyFilter // property-level filters
	Sort    []SortClause     // ordering (ignored when Text is set)
	Limit   int              // max results (0 = no limit)
}

// PropertyFilter matches entities by property value.
type PropertyFilter struct {
	Property string
	Value    string
	Op       FilterOp
}

// FilterOp defines how a property filter matches.
type FilterOp int

const (
	FilterEq        FilterOp = iota // exact match (default)
	FilterNe                        // not equal
	FilterContains                  // substring match
	FilterGt                        // greater than
	FilterLt                        // less than
	FilterGte                       // greater than or equal
	FilterLte                       // less than or equal
	FilterIn                        // value is one of a comma-separated set
	FilterExists                    // property is set (Value ignored)
	FilterNotExists                 // property is not set (Value ignored)
)

// SortClause defines a single sort dimension.
type SortClause struct {
	Field     string
	Direction SortDirection
}

// SortDirection is ascending or descending.
type SortDirection int

const (
	SortAsc SortDirection = iota
	SortDesc
)
