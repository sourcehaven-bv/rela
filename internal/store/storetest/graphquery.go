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

	t.Run("Props_equality", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "task", "T-1", map[string]any{"status": "doing"})
		seedEntityWithProps(t, s, "task", "T-2", map[string]any{"status": "todo"})
		seedEntityWithProps(t, s, "task", "T-3", map[string]any{"status": "doing"})

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "task",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "doing"},
			},
		})
		require.Equal(t, []string{"T-1", "T-3"}, got)
	})

	t.Run("Props_multiple_are_ANDed", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "task", "T-1", map[string]any{"status": "doing", "effort": "xs"})
		seedEntityWithProps(t, s, "task", "T-2", map[string]any{"status": "doing", "effort": "l"})
		seedEntityWithProps(t, s, "task", "T-3", map[string]any{"status": "todo", "effort": "xs"})

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "task",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "doing"},
				{Property: "effort", Op: store.PropEqual, Value: "xs"},
			},
		})
		require.Equal(t, []string{"T-1"}, got)
	})

	// Absent and present-but-empty MUST behave identically — the
	// emptiness contract from internal/propmatch, which internal/filter
	// shares. A backend that distinguishes them (e.g. jsonb `key missing`
	// vs `key: null`) fails here, which is the point.
	t.Run("Props_empty_matches_absent_and_blank", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "client", "C-absent", nil)
		seedEntityWithProps(t, s, "client", "C-blank", map[string]any{"billing_email": ""})
		seedEntityWithProps(t, s, "client", "C-set", map[string]any{"billing_email": "a@b.c"})

		isEmpty := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "client",
			Props: []store.PropPredicate{
				{Property: "billing_email", Op: store.PropEqual},
			},
		})
		require.Equal(t, []string{"C-absent", "C-blank"}, isEmpty,
			"absent and present-but-empty must both count as empty")

		notEmpty := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "client",
			Props: []store.PropPredicate{
				{Property: "billing_email", Op: store.PropNotEqual},
			},
		})
		require.Equal(t, []string{"C-set"}, notEmpty)
	})

	// An unset property must NOT satisfy an exclusion filter, or every
	// `prop!=value` query silently widens to include unset rows.
	t.Run("Props_exclusion_does_not_widen", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "task", "T-doing", map[string]any{"status": "doing"})
		seedEntityWithProps(t, s, "task", "T-todo", map[string]any{"status": "todo"})
		seedEntityWithProps(t, s, "task", "T-unset", nil)

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "task",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropNotEqual, Value: "doing"},
			},
		})
		require.Equal(t, []string{"T-todo"}, got,
			"an entity with no status is not in the 'status != doing' population")
	})

	// Value-shape parity. The naive backend compares Go values via
	// propmatch; pgstore compares jsonb in SQL. Every shape a property
	// can hold must land on the same answer in both, or a query means
	// different things per backend — the failure CLAUDE.md's parity rule
	// exists to prevent. Lists are the shape most likely to break: `->>`
	// renders an array as JSON text, so a naive `slices.Contains` match
	// and a SQL `= 'a'` disagree unless the SQL branches on type.
	t.Run("Props_value_shapes", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "doc", "D-str", map[string]any{"p": "a"})
		seedEntityWithProps(t, s, "doc", "D-otherstr", map[string]any{"p": "z"})
		seedEntityWithProps(t, s, "doc", "D-blank", map[string]any{"p": ""})
		seedEntityWithProps(t, s, "doc", "D-absent", nil)
		seedEntityWithProps(t, s, "doc", "D-emptylist", map[string]any{"p": []any{}})
		seedEntityWithProps(t, s, "doc", "D-list", map[string]any{"p": []any{"a", "b"}})
		seedEntityWithProps(t, s, "doc", "D-otherlist", map[string]any{"p": []any{"x"}})
		seedEntityWithProps(t, s, "doc", "D-int", map[string]any{"p": 3})
		seedEntityWithProps(t, s, "doc", "D-bool", map[string]any{"p": true})

		shapeCases := []struct {
			name string
			pred store.PropPredicate
			want []string
		}{
			{
				// An empty list is as empty as an absent key: a
				// multi-select with nothing selected reads as unset.
				name: "is-empty covers absent, blank and empty list",
				pred: store.PropPredicate{Property: "p", Op: store.PropEqual},
				want: []string{"D-absent", "D-blank", "D-emptylist"},
			},
			{
				name: "is-not-empty is the exact complement",
				pred: store.PropPredicate{Property: "p", Op: store.PropNotEqual},
				want: []string{"D-bool", "D-int", "D-list", "D-otherlist", "D-otherstr", "D-str"},
			},
			{
				// Multi-select: a list matches when ANY element does.
				name: "equality matches scalar and list membership",
				pred: store.PropPredicate{Property: "p", Op: store.PropEqual, Value: "a"},
				want: []string{"D-list", "D-str"},
			},
			{
				// Exclusion must not sweep in the empty rows.
				name: "exclusion excludes matches and empties",
				pred: store.PropPredicate{Property: "p", Op: store.PropNotEqual, Value: "a"},
				want: []string{"D-bool", "D-int", "D-otherlist", "D-otherstr"},
			},
			{
				name: "int compares by text form",
				pred: store.PropPredicate{Property: "p", Op: store.PropEqual, Value: "3"},
				want: []string{"D-int"},
			},
			{
				name: "bool compares by text form",
				pred: store.PropPredicate{Property: "p", Op: store.PropEqual, Value: "true"},
				want: []string{"D-bool"},
			},
		}
		for _, tc := range shapeCases {
			t.Run(tc.name, func(t *testing.T) {
				got := runGraphQuery(t, s, store.GraphQuery{
					EntityType: "doc",
					Props:      []store.PropPredicate{tc.pred},
				})
				require.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("Props_combine_with_relation_predicate", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "ticket", "TKT-1", map[string]any{"status": "ready"})
		seedEntityWithProps(t, s, "ticket", "TKT-2", map[string]any{"status": "done"})
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")
		mustRel(t, s, "alice", "owns", "TKT-2")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "ready"},
			},
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	// Negation: "entities with no outbound edge of this type at all".
	// Endpoints empty means "any endpoint", so this is a pure absence
	// query — the analyze_orphans / missing-billing-contact shape.
	t.Run("Negate_outbound_absence", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3")
		seedGraphQueryEntities(t, s, "feature", "FEAT-1")
		mustRel(t, s, "TKT-1", "implements", "FEAT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"implements"},
				Negate:  true,
			},
		})
		require.Equal(t, []string{"TKT-2", "TKT-3"}, got)
	})

	t.Run("Negate_inbound_absence", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				OfTypes: []string{"owns"},
				Negate:  true,
			},
		})
		require.Equal(t, []string{"TKT-2"}, got)
	})

	// Negation with named endpoints is narrower than absence: TKT-2 has
	// an owner, just not alice, so it matches "not owned by alice".
	t.Run("Negate_with_named_endpoints", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedGraphQueryEntities(t, s, "person", "alice", "bob")
		mustRel(t, s, "alice", "owns", "TKT-1")
		mustRel(t, s, "bob", "owns", "TKT-2")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
				Negate:    true,
			},
		})
		require.Equal(t, []string{"TKT-2"}, got)
	})

	// A non-negated predicate with no endpoints is an EXISTENCE query —
	// the mirror of the absence case above. Pinned so the "empty
	// Endpoints means any" rule holds in both polarities.
	t.Run("AnyEndpoint_existence", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedGraphQueryEntities(t, s, "feature", "FEAT-1")
		mustRel(t, s, "TKT-1", "implements", "FEAT-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"implements"},
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("Props_and_Negate_in_GraphCount", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "task", "T-1", map[string]any{"status": "doing"})
		seedEntityWithProps(t, s, "task", "T-2", map[string]any{"status": "todo"})
		seedEntityWithProps(t, s, "task", "T-3", map[string]any{"status": "doing"})

		matched, total, err := s.GraphCount(context.Background(), store.GraphQuery{
			EntityType: "task",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "doing"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, 2, matched)
		require.Equal(t, 3, total, "total ignores predicates")
	})

	t.Run("Props_in_MatchingIDs", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "task", "T-1", map[string]any{"status": "doing"})
		seedEntityWithProps(t, s, "task", "T-2", map[string]any{"status": "todo"})

		got, err := s.MatchingIDs(context.Background(), store.GraphQuery{
			EntityType: "task",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "doing"},
			},
		}, []string{"T-1", "T-2"})
		require.NoError(t, err)
		require.Equal(t, map[string]bool{"T-1": true, "T-2": false}, got)
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

	t.Run("MatchingIDs_filters_to_candidate_set", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3")
		seedGraphQueryEntities(t, s, "person", "alice")
		mustRel(t, s, "alice", "owns", "TKT-1")
		mustRel(t, s, "alice", "owns", "TKT-2")

		got, err := s.MatchingIDs(ctx(), store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &store.RelationPredicate{
				Endpoints: []string{"alice"},
				OfTypes:   []string{"owns"},
			},
		}, []string{"TKT-1", "TKT-2", "TKT-3"})
		require.NoError(t, err)
		require.Equal(t, map[string]bool{"TKT-1": true, "TKT-2": true, "TKT-3": false}, got)
	})

	t.Run("MatchingIDs_returns_all_input_ids", func(t *testing.T) {
		// Contract: every input id is present in the result map.
		// Callers distinguish "absent because no-match" (false in map)
		// from "not in store at all" (also false). An implementation
		// that silently drops ids would break the single-entity
		// visibility check.
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1")

		got, err := s.MatchingIDs(ctx(), store.GraphQuery{
			EntityType: "ticket",
		}, []string{"TKT-1", "nonexistent"})
		require.NoError(t, err)
		require.Equal(t, map[string]bool{"TKT-1": true, "nonexistent": false}, got)
	})

	t.Run("MatchingIDs_empty_input_returns_empty_map", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")

		got, err := s.MatchingIDs(ctx(), store.GraphQuery{
			EntityType: "ticket",
		}, nil)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("MatchingIDs_wrong_type_does_not_match", func(t *testing.T) {
		// A candidate id that exists in the store but as a different
		// type must map to false — the predicate is type-scoped.
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1")
		seedGraphQueryEntities(t, s, "feature", "FEAT-1")

		got, err := s.MatchingIDs(ctx(), store.GraphQuery{
			EntityType: "ticket",
		}, []string{"TKT-1", "FEAT-1"})
		require.NoError(t, err)
		require.Equal(t, map[string]bool{"TKT-1": true, "FEAT-1": false}, got)
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

// seedEntityWithProps creates one entity carrying the given properties.
// A nil value seeds an ABSENT key; an empty string seeds a
// present-but-empty one — the two states the emptiness contract says
// must behave identically.
func seedEntityWithProps(t *testing.T, s store.Store, typ, id string, props map[string]any) {
	t.Helper()
	e := entity.New(id, typ)
	for k, v := range props {
		if v == nil {
			continue // absent key
		}
		e.Properties[k] = v
	}
	require.NoError(t, s.CreateEntity(ctx(), e), "create %s/%s", typ, id)
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
