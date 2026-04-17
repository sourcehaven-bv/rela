// Package storetest provides a conformance test suite for store.Store
// implementations. Each implementation wires the suite via a Factory
// function that returns a fresh, empty store.
package storetest

import (
	"context"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/require"
)

// Factory returns a fresh, empty store for each test.
type Factory func(t *testing.T) store.Store

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
func RunAll(t *testing.T, f Factory) {
	t.Run("Entity", func(t *testing.T) { RunEntityTests(t, f) })
	t.Run("Relation", func(t *testing.T) { RunRelationTests(t, f) })
	t.Run("Query", func(t *testing.T) { RunQueryTests(t, f) })
	t.Run("Search", func(t *testing.T) { RunSearchTests(t, f) })
	t.Run("Attachment", func(t *testing.T) { RunAttachmentTests(t, f) })
	t.Run("Watcher", func(t *testing.T) { RunWatcherTests(t, f) })
	t.Run("Validation", func(t *testing.T) { RunValidationTests(t, f) })
}
