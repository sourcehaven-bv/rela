package storetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunRelatedTests is the conformance suite for [store.GraphQuery.Related],
// the caller-supplied conjunction of directed relation predicates
// (TKT-XKCNCL).
//
// The scenarios that matter to a backend author:
//
//   - Incoming reads the entity's incoming edges and false its outgoing ones,
//     exactly as HasInbound and HasOutbound do
//   - entries are ANDed with each other and with HasInbound: an entry never
//     replaces or widens the slot the ACL read gate occupies
//   - Negate, paging and the scoped count honor every entry
//   - the EndpointMatch shape bound applies to entries too
//
// Run via [RunAll].
func RunRelatedTests(t *testing.T, f Factory) {
	t.Helper()

	// seed builds:
	//   TKT-1 -implements-> FEAT-1, TKT-1 -caused-by-> CON-open
	//   TKT-2 -implements-> FEAT-2, TKT-2 -caused-by-> CON-done
	//   TKT-3 -caused-by-> CON-open
	//   USR-1 -owns-> FEAT-1, USR-1 -owns-> FEAT-3
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3")
		seedGraphQueryEntities(t, s, "feature", "FEAT-1", "FEAT-2", "FEAT-3")
		seedGraphQueryEntities(t, s, "user", "USR-1")
		seedEntityWithProps(t, s, "concept", "CON-open", map[string]any{"status": "open"})
		seedEntityWithProps(t, s, "concept", "CON-done", map[string]any{"status": "done"})
		mustRel(t, s, "TKT-1", "implements", "FEAT-1")
		mustRel(t, s, "TKT-2", "implements", "FEAT-2")
		mustRel(t, s, "TKT-1", "caused-by", "CON-open")
		mustRel(t, s, "TKT-2", "caused-by", "CON-done")
		mustRel(t, s, "TKT-3", "caused-by", "CON-open")
		mustRel(t, s, "USR-1", "owns", "FEAT-1")
		mustRel(t, s, "USR-1", "owns", "FEAT-3")
	}
	implementedBy := func(incoming bool) store.DirectedRelation {
		return store.DirectedRelation{
			Incoming: incoming,
			Pred:     store.RelationPredicate{OfTypes: []string{"implements"}},
		}
	}
	causedByOpen := store.DirectedRelation{Pred: store.RelationPredicate{
		OfTypes: []string{"caused-by"},
		EndpointMatch: &store.EndpointPredicate{Props: []store.PropPredicate{
			{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
		}},
	}}

	t.Run("Incoming_reads_incoming_edges", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "feature", Related: []store.DirectedRelation{implementedBy(true)},
		})
		require.Equal(t, []string{"FEAT-1", "FEAT-2"}, got)
	})

	t.Run("outgoing_reads_outgoing_edges", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// A feature has no OUTGOING implements edge. A backend that read the
		// zero direction as "both" would return FEAT-1 and FEAT-2 here.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "feature", Related: []store.DirectedRelation{implementedBy(false)},
		})
		require.Empty(t, got)

		got = runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket", Related: []store.DirectedRelation{implementedBy(false)},
		})
		require.Equal(t, []string{"TKT-1", "TKT-2"}, got)
	})

	t.Run("entries_are_a_conjunction", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// TKT-1 satisfies both; TKT-2 only implements; TKT-3 only the cause.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			Related:    []store.DirectedRelation{implementedBy(false), causedByOpen},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("ANDed_with_HasInbound_in_the_same_direction", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// HasInbound stands in for the ACL read gate ("owned by USR-1"):
		// FEAT-1 and FEAT-3. The entry narrows to features someone
		// implements: FEAT-1 and FEAT-2. Only FEAT-1 is both. FEAT-2 in the
		// result would mean the entry replaced the gate; FEAT-3 would mean
		// the entry was dropped.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "feature",
			HasInbound: &store.RelationPredicate{OfTypes: []string{"owns"}, Endpoints: []string{"USR-1"}},
			Related:    []store.DirectedRelation{implementedBy(true)},
		})
		require.Equal(t, []string{"FEAT-1"}, got)
	})

	t.Run("Negate_inverts_one_entry", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		negated := causedByOpen
		negated.Pred.Negate = true
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			Related:    []store.DirectedRelation{implementedBy(false), negated},
		})
		require.Equal(t, []string{"TKT-2"}, got)
	})

	t.Run("paging_count_and_MatchingIDs_honor_entries", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		q := store.GraphQuery{
			EntityType: "feature",
			Related:    []store.DirectedRelation{implementedBy(true)},
			// No feature holds "rank", so every row ties and the id
			// tiebreak orders the page.
			OrderBy: []store.OrderSpec{{Property: "rank"}},
			Limit:   1,
			Offset:  1,
		}
		var page []string
		for h, err := range store.GraphQueryHeaders(context.Background(), s, q) {
			require.NoError(t, err)
			page = append(page, h.ID)
		}
		require.Equal(t, []string{"FEAT-2"}, page)

		n, err := store.CountMatched(context.Background(), s, q)
		require.NoError(t, err)
		require.Equal(t, 2, n, "the count ignores paging but honors the entry")

		ids, err := s.MatchingIDs(context.Background(), q, []string{"FEAT-1", "FEAT-2", "FEAT-3"})
		require.NoError(t, err)
		// A backend may report a non-match as false or omit it; only the
		// truth of each id is the contract.
		require.True(t, ids["FEAT-1"])
		require.True(t, ids["FEAT-2"])
		require.False(t, ids["FEAT-3"])
	})

	t.Run("nested_inheritance_in_an_entry_is_refused", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		q := store.GraphQuery{
			EntityType: "ticket",
			Related: []store.DirectedRelation{{Pred: store.RelationPredicate{
				OfTypes: []string{"implements"},
				EndpointMatch: &store.EndpointPredicate{
					HasInbound: &store.RelationPredicate{InheritThrough: []string{"owns"}},
				},
			}}},
		}
		for _, err := range s.GraphQuery(context.Background(), q) {
			require.Error(t, err)
			return
		}
		t.Fatal("expected the shape check to refuse the query")
	})
}
