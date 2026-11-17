package storetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunEdgePropsTests is the conformance suite for
// [store.RelationPredicate.EdgeProps]: a relation predicate that only counts
// an edge whose own properties match (TKT-TKSCSBT, conditional ACL grants).
//
// The cases a backend author must get right:
//
//   - an absent, empty or list value never satisfies an ordered comparison,
//     so a grant with no valid date fails closed
//   - the predicate applies to the named edge only, never to the
//     EntityInheritThrough containment hops that reach it
//   - Negate inverts the predicate including the edge condition
//
// Run via [RunAll].
func RunEdgePropsTests(t *testing.T, f Factory) {
	t.Helper()

	// TKT-1..TKT-6 each get one grants edge from USR-1 with a different
	// valid_until; TKT-7 has none and sits under TKT-1 through partOf, an
	// edge without the property.
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		seedGraphQueryEntities(t, s, "ticket", "TKT-1", "TKT-2", "TKT-3", "TKT-4", "TKT-5", "TKT-6", "TKT-7")
		seedGraphQueryEntities(t, s, "person", "USR-1")
		for id, v := range map[string]any{
			"TKT-1": "2026-12-31",
			"TKT-2": "2026-01-01",
			"TKT-3": "",
			"TKT-4": []any{"2026-12-31"},
			"TKT-5": "3",
		} {
			mustRelProps(t, s, "USR-1", "grants", id, map[string]any{"valid_until": v})
		}
		mustRelProps(t, s, "USR-1", "grants", "TKT-6", nil)
		mustRel(t, s, "TKT-7", "partOf", "TKT-1")
	}
	// active is the lowering of `edge.valid_until >= today` on 2026-11-16:
	// the shape guard keeps `3`, which byte-compares above any date, out.
	active := []store.PropPredicate{
		{Property: "valid_until", Op: store.PropIsDate},
		{Property: "valid_until", Op: store.PropGreaterEqual, Value: "2026-11-16"},
	}
	query := func(p store.RelationPredicate) store.GraphQuery {
		return store.GraphQuery{
			EntityType: "ticket",
			HasInbound: &p,
			Faces:      store.InWorld(store.TrivialScope()),
		}
	}

	t.Run("only_a_matching_edge_counts", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, query(store.RelationPredicate{
			Endpoints: []string{"USR-1"}, OfTypes: []string{"grants"}, EdgeProps: active,
		}))
		require.Equal(t, []string{"TKT-1"}, got)
	})

	t.Run("nil_EdgeProps_is_unconstrained", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, query(store.RelationPredicate{
			Endpoints: []string{"USR-1"}, OfTypes: []string{"grants"},
		}))
		require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4", "TKT-5", "TKT-6"}, got)
	})

	t.Run("applies_to_the_grant_edge_not_the_inherit_hop", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, query(store.RelationPredicate{
			Endpoints: []string{"USR-1"}, OfTypes: []string{"grants"}, EdgeProps: active,
			EntityInheritThrough: []string{"partOf"}, EntityDepth: 3,
		}))
		require.Equal(t, []string{"TKT-1", "TKT-7"}, got)
	})

	t.Run("Negate_inverts_the_edge_condition", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, query(store.RelationPredicate{
			Endpoints: []string{"USR-1"}, OfTypes: []string{"grants"}, EdgeProps: active, Negate: true,
		}))
		require.Equal(t, []string{"TKT-2", "TKT-3", "TKT-4", "TKT-5", "TKT-6", "TKT-7"}, got)
	})

	t.Run("IsDate_matches_the_shape_only", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		got := runGraphQuery(t, s, query(store.RelationPredicate{
			Endpoints: []string{"USR-1"}, OfTypes: []string{"grants"},
			EdgeProps: []store.PropPredicate{{Property: "valid_until", Op: store.PropIsDate}},
		}))
		require.Equal(t, []string{"TKT-1", "TKT-2"}, got)
	})

	t.Run("unknown_operator_is_refused", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		var err error
		for _, e := range s.GraphQuery(ctx(), query(store.RelationPredicate{
			OfTypes:   []string{"grants"},
			EdgeProps: []store.PropPredicate{{Property: "valid_until", Op: store.PropOp(99), Value: "x"}},
		})) {
			if e != nil {
				err = e
				break
			}
		}
		require.ErrorIs(t, err, store.ErrInvalidQuery)
	})
}

// mustRelProps creates an identity-scoped edge carrying props.
func mustRelProps(t *testing.T, s store.Store, from, relType, to string, props map[string]any) {
	t.Helper()
	_, err := s.CreateRelation(ctx(), entity.RelationKey{From: from, Type: relType, To: to},
		&store.RelationData{Properties: props})
	require.NoError(t, err, "%s --%s--> %s", from, relType, to)
}
