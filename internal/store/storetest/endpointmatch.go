package storetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunEndpointMatchTests is the conformance suite for
// [store.RelationPredicate.EndpointMatch] — filtering the candidate set by the
// TYPE and PROPERTIES of the entity on the far side of a relation, rather than
// by endpoint id (TKT-RELTRV).
//
// The scenarios that matter to a backend author, beyond the happy path:
//
//   - a dangling edge (endpoint row absent) is NOT a match, so a pushed-down
//     JOIN and a Go-side lookup agree
//   - EndpointMatch composes with Endpoints as a CONJUNCTION, not a widening
//   - Negate inverts the whole predicate including the endpoint filter
//   - a chained hop resolves against the endpoint, not the original candidate
//
// Run via [RunAll].
func RunEndpointMatchTests(t *testing.T, f Factory) {
	t.Helper()

	// seed builds: TKT-1 -caused-by-> CON-open, TKT-2 -caused-by-> CON-done,
	// TKT-3 -caused-by-> MISSING (dangling).
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3")
		seedEntityWithProps(t, s, "concept", "CON-open", map[string]any{"status": "open"})
		seedEntityWithProps(t, s, "concept", "CON-done", map[string]any{"status": "done"})
		mustRel(t, s, "TKT-1", "caused-by", "CON-open")
		mustRel(t, s, "TKT-2", "caused-by", "CON-done")
		mustRel(t, s, "TKT-3", "caused-by", "CON-missing")
	}

	t.Run("Props_filters_by_endpoint_property", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"caused-by"},
				EndpointMatch: &store.EndpointPredicate{
					Props: []store.PropPredicate{
						{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
					},
				},
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("EntityType_resolves_a_union_target", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedEntityWithProps(t, s, "concept", "CON-1", map[string]any{"status": "open"})
		seedEntityWithProps(t, s, "decision", "DEC-1", map[string]any{"status": "open"})
		mustRel(t, s, "TKT-1", "caused-by", "CON-1")
		mustRel(t, s, "TKT-2", "caused-by", "DEC-1")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes:       []string{"caused-by"},
				EndpointMatch: &store.EndpointPredicate{EntityType: "decision"},
			},
		})
		require.Equal(t, []string{"TKT-2"}, got)
	})

	t.Run("dangling_edge_is_not_a_match", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// TKT-3's endpoint row does not exist. An any-type, any-property
		// endpoint match must still exclude it: a pushed-down INNER JOIN
		// drops the row, and the Go path must agree.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes:       []string{"caused-by"},
				EndpointMatch: &store.EndpointPredicate{},
			},
		})
		require.Equal(t, []string{"TKT-1", "TKT-2"}, got)
	})

	t.Run("composes_with_Endpoints_as_conjunction", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// Endpoints names CON-done; the property filter names status=open.
		// Nothing satisfies both, so the result is empty — if the two were
		// OR-ed (or either were ignored) this would return a row.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes:   []string{"caused-by"},
				Endpoints: []string{"CON-done"},
				EndpointMatch: &store.EndpointPredicate{
					Props: []store.PropPredicate{
						{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
					},
				},
			},
		})
		require.Empty(t, got)
	})

	t.Run("Negate_inverts_including_the_endpoint_filter", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// "has no caused-by edge to an open concept" — TKT-2 (edge to a done
		// concept) and TKT-3 (dangling) both qualify.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"caused-by"},
				Negate:  true,
				EndpointMatch: &store.EndpointPredicate{
					Props: []store.PropPredicate{
						{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
					},
				},
			},
		})
		require.Equal(t, []string{"TKT-2", "TKT-3"}, got)
	})

	t.Run("chained_hop_resolves_against_the_endpoint", func(t *testing.T) {
		s := f(t)
		// TKT-1 -caused-by-> CON-1 -owned-by-> alice
		// TKT-2 -caused-by-> CON-2 -owned-by-> bob
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2")
		seedGraphQueryEntities(t, s, "concept", "CON-1", "CON-2")
		seedGraphQueryEntities(t, s, "person", "alice", "bob")
		mustRel(t, s, "TKT-1", "caused-by", "CON-1")
		mustRel(t, s, "TKT-2", "caused-by", "CON-2")
		mustRel(t, s, "CON-1", "owned-by", "alice")
		mustRel(t, s, "CON-2", "owned-by", "bob")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"caused-by"},
				EndpointMatch: &store.EndpointPredicate{
					EntityType: "concept",
					HasOutbound: &store.RelationPredicate{
						OfTypes:   []string{"owned-by"},
						Endpoints: []string{"alice"},
					},
				},
			},
		})
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("nil_EndpointMatch_is_unconstrained", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		// The pre-existing shape: a nil match must not start excluding the
		// dangling edge, or adding the field would change existing queries.
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType:  "ticket",
			HasOutbound: &store.RelationPredicate{OfTypes: []string{"caused-by"}},
		})
		require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3"}, got)
	})
}
