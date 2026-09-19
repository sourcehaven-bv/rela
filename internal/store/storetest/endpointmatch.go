package storetest

import (
	"context"
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

	// The ACL folds its read query into an endpoint match, and that query
	// carries InheritThrough / EntityInheritThrough whenever the policy
	// declares role inheritance. pgstore's nested emitter cannot express them,
	// so BOTH backends must refuse rather than one silently ignoring them —
	// an authorization predicate that evaporates on one backend is the defect
	// this case exists to catch.
	t.Run("nested_inheritance_expansion_is_refused_by_every_backend", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		err := graphQueryErr(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"caused-by"},
				EndpointMatch: &store.EndpointPredicate{
					EntityType: "concept",
					HasInbound: &store.RelationPredicate{
						Endpoints:      []string{"alice"},
						OfTypes:        []string{"owns"},
						InheritThrough: []string{"member-of"},
						Depth:          3,
					},
				},
			},
		})
		require.Error(t, err, "a nested inheritance expansion must be refused, not silently ignored")
	})

	// A NEGATED predicate inverts everything beneath it, so any guard that
	// renders an arm unsatisfiable instead of refusing would turn "has no
	// path to X" into "match everything". Pinned because a depth bound was
	// briefly implemented that way.
	t.Run("negated_chained_hop", func(t *testing.T) {
		s := f(t)
		// TKT-1 -> CON-open -owned-by-> alice ; TKT-2 -> CON-done (no owner)
		seed(t, s)
		mustRel(t, s, "CON-open", "owned-by", "alice")

		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"caused-by"},
				Negate:  true,
				EndpointMatch: &store.EndpointPredicate{
					EntityType: "concept",
					HasOutbound: &store.RelationPredicate{
						OfTypes: []string{"owned-by"}, Endpoints: []string{"alice"},
					},
				},
			},
		})
		// Only TKT-1 reaches an alice-owned concept, so the negation keeps the
		// other two. Critically it must NOT be all three.
		require.Equal(t, []string{"TKT-2", "TKT-3"}, got)
	})

	// The nesting bound must REFUSE, not render the too-deep arm
	// unsatisfiable: under an outer Negate an unsatisfiable arm inverts to
	// "match everything", turning a DoS guard into a row-gate bypass.
	t.Run("excessive_nesting_is_refused", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		inner := &store.EndpointPredicate{EntityType: "concept"}
		for range 12 {
			inner = &store.EndpointPredicate{
				EntityType:  "concept",
				HasOutbound: &store.RelationPredicate{OfTypes: []string{"caused-by"}, EndpointMatch: inner},
			}
		}
		err := graphQueryErr(t, s, store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				OfTypes: []string{"caused-by"}, EndpointMatch: inner,
			},
		})
		require.Error(t, err, "nesting past the cap must be refused")
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

// graphQueryErr drains a GraphQuery and returns the first error, or nil.
// Unlike runGraphQuery it does not fail the test — callers asserting a
// REFUSAL need the error rather than a fatal.
func graphQueryErr(t *testing.T, s store.Store, q store.GraphQuery) error {
	t.Helper()
	for _, err := range s.GraphQuery(context.Background(), q) {
		if err != nil {
			return err
		}
	}
	return nil
}
