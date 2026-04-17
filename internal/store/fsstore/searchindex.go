package fsstore

import "github.com/Sourcehaven-BV/rela/internal/entity"

// SearchIndex is a pluggable full-text search index. Implementations must be
// safe for concurrent use. When no SearchIndex is provided in Config, a
// default linear substring scanner is used.
type SearchIndex interface {
	// Index adds or updates an entity in the search index.
	Index(e *entity.Entity) error

	// Remove deletes an entity from the search index.
	Remove(id string) error

	// Search returns entity IDs matching the query text, ordered by relevance.
	// limit ≤ 0 means no limit.
	Search(text string, limit int) ([]string, error)

	// Persistent returns true if the index survives process restarts.
	// When true, the index is only rebuilt when entity files have changed.
	// When false, all entities are re-indexed on every startup.
	Persistent() bool

	// Close releases any resources held by the index.
	Close() error
}
