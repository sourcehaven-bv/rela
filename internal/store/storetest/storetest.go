// Package storetest provides a conformance test suite for store.Store
// implementations. Each implementation wires the suite via a Factory
// function that returns a fresh, empty store.
package storetest

import (
	"context"
	"iter"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Factory returns a fresh, empty store for each test.
type Factory func(t *testing.T) store.Store

// SearchFactory returns a fresh store and its associated searcher for each test.
// The store is used for seeding data; the searcher is used for queries.
type SearchFactory func(t *testing.T) (store.Store, search.Searcher)

// Capabilities declares which optional Store features an implementation
// supports. Conformance tests gate optional sections on these flags so
// implementations that omit a feature (e.g. attachments) can still run
// the rest of the suite.
type Capabilities struct {
	// Attachments indicates whether the store implements
	// AttachmentManager. When false, attachment-related conformance
	// tests are skipped.
	Attachments bool
}

func ctx() context.Context { return context.Background() }

// seedEntities populates a store with a standard set of entities.
func seedEntities(t *testing.T, s store.Store) {
	t.Helper()
	for _, e := range []*entity.Entity{
		func() *entity.Entity {
			e := entity.New("FEAT-001", "feature")
			e.SetString("title", "Login")
			e.SetString("status", "open")
			return e
		}(),
		func() *entity.Entity {
			e := entity.New("FEAT-002", "feature")
			e.SetString("title", "Logout")
			e.SetString("status", "done")
			return e
		}(),
		func() *entity.Entity {
			e := entity.New("FEAT-013", "feature")
			e.SetString("title", "Dashboard")
			e.SetString("status", "open")
			return e
		}(),
		func() *entity.Entity {
			e := entity.New("REQ-001", "requirement")
			e.SetString("title", "Must authenticate")
			e.SetString("status", "open")
			return e
		}(),
	} {
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
}

// seedSearchData populates a store with entities suited for search tests.
func seedSearchData(t *testing.T, s store.Store) {
	t.Helper()
	for _, e := range []*entity.Entity{
		func() *entity.Entity {
			e := entity.New("FEAT-001", "feature")
			e.SetString("title", "User Login")
			e.SetString("status", "open")
			e.SetString("priority", "high")
			return e
		}(),
		func() *entity.Entity {
			e := entity.New("FEAT-002", "feature")
			e.SetString("title", "User Logout")
			e.SetString("status", "done")
			e.SetString("priority", "low")
			return e
		}(),
		func() *entity.Entity {
			e := entity.New("REQ-001", "requirement")
			e.SetString("title", "Authentication Required")
			e.SetString("status", "open")
			e.Content = "All users must login before accessing the system"
			return e
		}(),
	} {
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
}

// collectIter drains an entity iterator into a slice.
func collectIter(t *testing.T, it iter.Seq2[*entity.Entity, error]) []*entity.Entity {
	t.Helper()
	var results []*entity.Entity
	for e, err := range it {
		require.NoError(t, err)
		results = append(results, e)
	}
	return results
}

// collectHits drains a search hit iterator into a slice.
func collectHits(t *testing.T, it iter.Seq2[search.Hit, error]) []search.Hit {
	t.Helper()
	var results []search.Hit
	for h, err := range it {
		require.NoError(t, err)
		results = append(results, h)
	}
	return results
}

// searchError drains a search iterator and returns the first error it
// yields (nil if the search succeeded). Used by conformance cases that
// assert a query is rejected rather than asserting on hits.
func searchError(it iter.Seq2[search.Hit, error]) error {
	for _, err := range it {
		if err != nil {
			return err
		}
	}
	return nil
}

// countRelations counts relations matching a query.
func countRelations(t *testing.T, s store.Store) int {
	t.Helper()
	n := 0
	for _, err := range s.ListRelations(ctx(), store.RelationQuery{}) {
		require.NoError(t, err)
		n++
	}
	return n
}

// RunAll runs the full conformance suite.
// The optional SearchFactory is used for search tests; if nil, search tests are skipped.
// Capabilities gates optional feature tests (e.g. attachments).
func RunAll(t *testing.T, f Factory, sf SearchFactory, caps Capabilities) {
	t.Run("Entity", func(t *testing.T) { RunEntityTests(t, f) })
	t.Run("Relation", func(t *testing.T) { RunRelationTests(t, f) })
	t.Run("Query", func(t *testing.T) { RunQueryTests(t, f) })
	t.Run("GraphQuery", func(t *testing.T) { RunGraphQueryTests(t, f) })
	t.Run("Pagination", func(t *testing.T) { RunPaginationTests(t, f) })
	if sf != nil {
		t.Run("Search", func(t *testing.T) { RunSearchTests(t, sf) })
	}
	if caps.Attachments {
		t.Run("Attachment", func(t *testing.T) { RunAttachmentTests(t, f) })
	}
	t.Run("Watcher", func(t *testing.T) { RunWatcherTests(t, f) })
	t.Run("Validation", func(t *testing.T) { RunValidationTests(t, f) })
}
