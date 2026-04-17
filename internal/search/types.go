package search

import (
	"context"
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
