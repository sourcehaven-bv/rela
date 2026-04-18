package filter

import (
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Record is a lightweight property bag used for filtering and sorting.
// Callers construct Records from whatever entity type they have.
// For sort functions that need to reorder the caller's original slice,
// use the Accessor pattern with the generic sort functions.
type Record struct {
	ID         string
	Type       string
	Properties map[string]interface{}
	ModifiedAt time.Time // optional; used by "modified" virtual sort property
}

// Accessor extracts filter-relevant fields from any entity-like type.
// This allows sort/filter functions to work with any concrete type
// without requiring callers to convert their slices.
type Accessor[T any] func(T) Record

// SortSpec is an alias for metamodel.SortSpec so filter and metamodel share
// a single authoritative sort-criterion type.
type SortSpec = metamodel.SortSpec
