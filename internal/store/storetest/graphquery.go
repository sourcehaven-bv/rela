package storetest

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunGraphQueryTests is the conformance suite for [store.GraphQueryer].
// Pins the contract every backend implementation must honor —
// regardless of whether it delegates to graphquerynaive or pushes the
// query into the underlying engine. New implementations get the same
// expectations for free.
//
// Each subtest seeds a small graph, runs a GraphQuery, and asserts on
// the (id-sorted) result set. The scenarios cover:
//
//   - direct inbound match (no transitive expansion)
//   - direct outbound match
//   - InheritThrough endpoint-side expansion (groups-shaped)
//   - EntityInheritThrough entity-side expansion (containment-shaped)
//   - both expansions composed
//   - OfTypes filter
//   - cycle / self-loop termination
//   - depth-cap truncation
//   - GraphCount returning (matched, total)
//
// Run via [RunAll] alongside the other conformance suites.
func RunGraphQueryTests(t *testing.T, f Factory) {
	t.Helper()

	t.Run("HasInbound_direct", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")
		mustRel(t, s, "alice", "owns", "TKT-3")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
			},
		})
		require.Equal(t, []string{"TKT-1", "TKT-3"}, got)
	})

	t.Run("HasOutbound_direct", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedGraphQueryEntities(t, s, "feature", "FEAT-1")
		mustRel(t, s, "TKT-1", "implements", "FEAT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				Endpoints: []string{"FEAT-1"},
				OfTypes:   []string{"implements"},
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("InheritThrough_endpoint_expansion", func(t *testing.T) {
		// alice in group engineering; engineering has owns→TKT-1.
		// Without InheritThrough alice has no direct edge to TKT-1.
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1")
		seedGraphQueryEntities(t, s, "person", "alice")
		seedGraphQueryEntities(t, s, "team", "engineering")
		mustRel(t, s, "alice", "member-of", "engineering")
		mustRel(t, s, "engineering", "owns", "TKT-1")

		// Without expansion: alice owns nothing.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
			},
		})
		require.Empty(t, got, "without InheritThrough, no expansion")

		// With expansion: engineering is reachable from alice.
		got = runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints:      []string{"alice"},
				OfTypes:        []string{"owns"},
				InheritThrough: []string{"member-of"},
				Depth:          3,
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("EntityInheritThrough_entity_expansion", func(t *testing.T) {
		// D-secret belongs-to F-eng. alice owns F-eng. With
		// EntityInheritThrough, D-secret's ancestor F-eng surfaces
		// the inbound owns.
		s := f(t)
		seedGraphQueryEntities(t, s, "document", "D-secret")
		seedGraphQueryEntities(t, s, "folder", "F-eng")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "D-secret", "belongs-to", "F-eng")
		mustRel(t, s, "alice", "owns", "F-eng")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "document",
			HasInbound: &store.RelationPredicate{
				Endpoints:            []string{"alice"},
				OfTypes:              []string{"owns"},
				EntityInheritThrough: []string{"belongs-to"},
				EntityDepth:          3,
			},
		})
		require.Equal(t, []string{"D-secret"}, got)
	})

	t.Run("Both_expansions_compose", func(t *testing.T) {
		// alice → engineering (group) → owns F-eng → contains D-secret.
		s := f(t)
		seedGraphQueryEntities(t, s, "document", "D-secret")
		seedGraphQueryEntities(t, s, "folder", "F-eng")
		seedGraphQueryEntities(t, s, "person", "alice")
		seedGraphQueryEntities(t, s, "team", "engineering")
		mustRel(t, s, "alice", "member-of", "engineering")
		mustRel(t, s, "engineering", "owns", "F-eng")
		mustRel(t, s, "D-secret", "belongs-to", "F-eng")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "document",
			HasInbound: &store.RelationPredicate{
				Endpoints:            []string{"alice"},
				OfTypes:              []string{"owns"},
				InheritThrough:       []string{"member-of"},
				Depth:                3,
				EntityInheritThrough: []string{"belongs-to"},
				EntityDepth:          3,
			},
		})
		require.Equal(t, []string{"D-secret"}, got)
	})

	t.Run("OfTypes_filter", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")
		mustRel(t, s, "alice", "watches", "TKT-2")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
			},
		})
		require.Equal(t, []string{"TKT-1"}, got, "watches must not match")
	})

	t.Run("SelfLoop_terminates", func(t *testing.T) {
		// alice → member-of → alice. Walk must terminate.
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "member-of", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints:      []string{"alice"},
				OfTypes:        []string{"owns"},
				InheritThrough: []string{"member-of"},
				Depth:          5,
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("Cycle_terminates", func(t *testing.T) {
		// A → B → C → A via member-of. Walk from A must hit {A,B,C}
		// and stop; the C→A back-edge must not cause infinite loop.
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1")
		seedGraphQueryEntities(t, s, "team", "A", "B", "C")
		mustRel(t, s, "A", "member-of", "B")
		mustRel(t, s, "B", "member-of", "C")
		mustRel(t, s, "C", "member-of", "A")
		mustRel(t, s, "C", "owns", "TKT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints:      []string{"A"},
				OfTypes:        []string{"owns"},
				InheritThrough: []string{"member-of"},
				Depth:          5,
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("Depth_zero_is_no_op", func(t *testing.T) {
		// Depth=0 disables expansion even if InheritThrough is set —
		// only the direct seed matches.
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1")
		seedGraphQueryEntities(t, s, "person", "alice")
		seedGraphQueryEntities(t, s, "team", "engineering")
		mustRel(t, s, "alice", "member-of", "engineering")
		mustRel(t, s, "engineering", "owns", "TKT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints:      []string{"alice"},
				OfTypes:        []string{"owns"},
				InheritThrough: []string{"member-of"},
				Depth:          0,
			},
		})
		require.Empty(t, got, "Depth=0 must not expand")
	})

	t.Run("GraphCount_matched_and_total", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")
		mustRel(t, s, "alice", "owns", "TKT-2")

		matched, total, err := s.GraphCount(ctx(), store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, 2, matched, "2 of 3 tickets are alice-owned")
		require.Equal(t, 3, total, "3 tickets exist regardless of predicate")
	})
}

// seedGraphQueryEntities creates entities of the given type with the
// given IDs.
func seedGraphQueryEntities(t *testing.T, s store.Store, typ string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		e := entity.New(id, typ)
		require.NoError(t, s.CreateEntity(ctx(), e), "create %s/%s", typ, id)
	}
}

// mustRel creates a relation; fails the test on error.
func mustRel(t *testing.T, s store.Store, from, relType, to string) {
	t.Helper()
	_, err := s.CreateRelation(ctx(), from, relType, to, nil)
	require.NoError(t, err, "%s --%s--> %s", from, relType, to)
}

// runGraphQuery runs q and returns matched entity IDs in sorted order.
func runGraphQuery(t *testing.T, s store.Store, q store.GraphQuery) []string {
	t.Helper()
	var ids []string
	for e, err := range s.GraphQuery(context.Background(), q) {
		require.NoError(t, err)
		ids = append(ids, e.ID)
	}
	slices.Sort(ids)
	return ids
}
