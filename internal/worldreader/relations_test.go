package worldreader_test

import (
	"context"
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// recordingLister captures the queries issued so a test can assert on the
// FromPointer the dispatch chose — the distinction under test is between
// nil and a pointer-to-zero, which no assertion on the RESULTS could
// detect when a fixture happens to have only default-tail edges.
type recordingLister struct {
	queries []store.RelationQuery
	rels    []*entity.Relation
}

func (r *recordingLister) ListRelations(
	_ context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	r.queries = append(r.queries, q)
	rels := r.rels
	return func(yield func(*entity.Relation, error) bool) {
		for _, rel := range rels {
			if !yield(rel, nil) {
				return
			}
		}
	}
}

// contentTypes classifies the named types as content-scoped; everything
// else is identity-scoped (the default, which keeps pointerless projects
// unchanged).
type contentTypes map[string]bool

func (c contentTypes) IsContentScoped(relType string) bool { return c[relType] }

// TestNeighbors_FallbackTrap_ZeroPointerIsNotNil is the Q4 fallback trap
// (RR-CGRV0X follow-through 1), and it is the reason the dispatch issues
// two queries instead of one.
//
// When a prime is reached by `otherwise: default` — or by rule 1 — its
// pointer IS the zero Pointer. As a store.RelationQuery.FromPointer
// VALUE the zero pointer does NOT mean "unfiltered": it means
// default-tail-ONLY, a strictly different filter from nil. The two are
// indistinguishable in a debugger (both print as the empty pointer),
// so the invariant has to be pinned on the QUERY, not on the results.
//
// Getting this wrong in the identity direction hides an entity's role and
// containment edges; getting it wrong in the content direction shows a
// non-prime state's edges.
func TestNeighbors_FallbackTrap_ZeroPointerIsNotNil(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  worldreader.Resolved
	}{
		{
			name: "fallback to default state",
			res: worldreader.Resolved{
				Entity: entity.New("PAGE-1", "page"),
				// Zero pointer, reached via the fallback.
				Via:   worldreader.RuleFallbackDefault,
				Found: true,
			},
		},
		{
			name: "unscoped type (rule 1)",
			res: worldreader.Resolved{
				Entity: entity.New("PAGE-1", "page"),
				Via:    worldreader.RuleUnscoped,
				Found:  true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lister := &recordingLister{}
			rr, err := worldreader.NewRelationReader(lister, contentTypes{})
			require.NoError(t, err)

			_, err = rr.Neighbors(context.Background(), tc.res, store.DirectionOutgoing)
			require.NoError(t, err)
			require.Len(t, lister.queries, 2, "one identity query + one content query")

			identityQ, contentQ := lister.queries[0], lister.queries[1]

			assert.Nil(t, identityQ.FromPointer,
				"identity edges must query with a NIL tail: a pointer-to-zero would "+
					"filter to default-tail edges only and hide identity edges")

			require.NotNil(t, contentQ.FromPointer,
				"content edges must query with the prime's pointer BY VALUE, not nil")
			assert.Equal(t, entity.Pointer(""), *contentQ.FromPointer,
				"the fallback prime's pointer is the zero pointer, and it must be "+
					"passed as a value so the filter means default-tail-only")
		})
	}
}

// TestNeighbors_ContentQueryUsesThePrimesPointer pins the non-fallback
// half: when the prime is a real chain coordinate, content edges are
// scoped to THAT state while identity edges stay unfiltered.
func TestNeighbors_ContentQueryUsesThePrimesPointer(t *testing.T) {
	lister := &recordingLister{}
	rr, err := worldreader.NewRelationReader(lister, contentTypes{})
	require.NoError(t, err)

	res := worldreader.Resolved{
		Entity:  entity.New("PAGE-1", "page"),
		Pointer: entity.Pointer("published"),
		Via:     worldreader.RuleChain,
		Found:   true,
	}
	_, err = rr.Neighbors(context.Background(), res, store.DirectionOutgoing)
	require.NoError(t, err)
	require.Len(t, lister.queries, 2)

	assert.Nil(t, lister.queries[0].FromPointer, "identity tail stays nil for every prime")
	require.NotNil(t, lister.queries[1].FromPointer)
	assert.Equal(t, entity.Pointer("published"), *lister.queries[1].FromPointer)
}

// TestNeighbors_MergesByScopeClass pins that each edge is kept under the
// query whose scope class matches its type. Both queries over-return —
// the nil-tail query also matches content edges — so a naive union would
// duplicate every identity edge and leak non-prime content edges.
func TestNeighbors_MergesByScopeClass(t *testing.T) {
	identityRel := &entity.Relation{From: "PAGE-1", Type: "owned-by", To: "USER-1"}
	contentRel := &entity.Relation{From: "PAGE-1", Type: "mentions", To: "PAGE-2"}
	lister := &recordingLister{rels: []*entity.Relation{identityRel, contentRel}}

	rr, err := worldreader.NewRelationReader(lister, contentTypes{"mentions": true})
	require.NoError(t, err)

	res := worldreader.Resolved{
		Entity:  entity.New("PAGE-1", "page"),
		Pointer: entity.Pointer("published"),
		Via:     worldreader.RuleChain,
		Found:   true,
	}
	got, err := rr.Neighbors(context.Background(), res, store.DirectionOutgoing)
	require.NoError(t, err)

	// Each edge appears exactly once, under its own class.
	require.Len(t, got, 2, "each edge kept once, no duplicates from the two queries")
	assert.Equal(t, "owned-by", got[0].Type, "identity edge from the nil-tail query")
	assert.Equal(t, "mentions", got[1].Type, "content edge from the pointer query")
}

// TestNeighbors_ExcludedEntityHasNoEdges: an entity the world excludes has
// no edges IN THIS WORLD. Returning storage's edges would leak exactly
// the existence the exclusion withholds.
func TestNeighbors_ExcludedEntityHasNoEdges(t *testing.T) {
	lister := &recordingLister{rels: []*entity.Relation{
		{From: "PAGE-1", Type: "owned-by", To: "USER-1"},
	}}
	rr, err := worldreader.NewRelationReader(lister, contentTypes{})
	require.NoError(t, err)

	got, err := rr.Neighbors(context.Background(),
		worldreader.Resolved{Via: worldreader.RuleExcluded}, store.DirectionOutgoing)
	require.NoError(t, err)

	assert.Empty(t, got, "an excluded entity contributes no edges")
	assert.Empty(t, lister.queries, "and no query is issued at all")
}

// TestNeighbors_DirectionBothMatchesEitherEndpoint pins the endpoint
// selection. DirectionBoth is the ZERO value of store.Direction, so
// setting From alone would silently narrow every unspecified query to
// outgoing-only — a wrong-by-default failure.
func TestNeighbors_DirectionBothMatchesEitherEndpoint(t *testing.T) {
	lister := &recordingLister{}
	rr, err := worldreader.NewRelationReader(lister, contentTypes{})
	require.NoError(t, err)

	res := worldreader.Resolved{Entity: entity.New("PAGE-1", "page"), Found: true}
	_, err = rr.Neighbors(context.Background(), res, store.DirectionBoth)
	require.NoError(t, err)

	for _, q := range lister.queries {
		assert.Equal(t, "PAGE-1", q.EntityID, "DirectionBoth must filter on either endpoint")
		assert.Empty(t, q.From, "From alone would narrow both to outgoing-only")
		assert.Empty(t, q.To)
	}
}

func TestNewRelationReader_RejectsNilCollaborators(t *testing.T) {
	_, err := worldreader.NewRelationReader(nil, contentTypes{})
	require.Error(t, err, "a nil lister must be rejected, not silently accepted")

	_, err = worldreader.NewRelationReader(&recordingLister{}, nil)
	require.Error(t, err, "a nil classifier would make every type identity-scoped")
}
