package visibility

import (
	"context"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DECORATOR/PUSHDOWN PARITY for worlds (TKT-WAV8XP PR-D).
//
// A world reaches the store two ways, and both are required:
//
//   - as a DECORATOR, on the read paths that go through a reader; and
//   - as a FIELD ON THE QUERY, because listPushdown composes a GraphQuery
//     and hands it to the RAW store, reaching past every decorator.
//
// The two must agree. They did not in PR-B: `GraphQuery.World` existed and
// was never populated on this path, so a world-scoped list silently
// degraded to the default world — and did so for exactly the ACL-GATED
// principals, because the AllowAll branch takes the EntityQuery path and
// only a principal WITH a policy query reaches the GraphQuery branch
// (RR-GQWRLD).
//
// That is why these tests exercise the COMPOSED-QUERY branch specifically.
// A parity test covering only AllowAll would have passed throughout the
// window the bug was live.

// worldCapturingSpy records the queries it is handed so a test can assert
// the world ARRIVED. Asserting on returned rows cannot do that: the spy
// returns whatever rows it was seeded with regardless of scope, and in a
// real store a wrongly-defaulted world still returns plausible rows —
// which is precisely why the original bug was invisible.
type worldCapturingSpy struct {
	store.Store
	graphQueries  []store.GraphQuery
	entityQueries []store.EntityQuery
	rows          []*entity.Entity
}

func (s *worldCapturingSpy) GraphQuery(
	_ context.Context, q store.GraphQuery,
) iter.Seq2[*entity.Entity, error] {
	s.graphQueries = append(s.graphQueries, q)
	return s.seq()
}

func (s *worldCapturingSpy) ListEntities(
	_ context.Context, q store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	s.entityQueries = append(s.entityQueries, q)
	return s.seq()
}

func (s *worldCapturingSpy) GraphCount(
	context.Context, store.GraphQuery,
) (matched, total int, err error) {
	return 0, 0, nil
}

func (s *worldCapturingSpy) MatchingIDs(
	context.Context, store.GraphQuery, []string,
) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (s *worldCapturingSpy) seq() iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range s.rows {
			if !yield(e, nil) {
				return
			}
		}
	}
}

func testWorld() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {
			Chain:    []entity.Pointer{"published"},
			Fallback: store.FallbackExclude,
		},
	})
}

// TestWorldParity_ComposedQueryBranchCarriesTheWorld is the regression
// test for RR-GQWRLD. A principal WITH a policy query takes the
// GraphQuery branch, and the world must travel onto that query.
func TestWorldParity_ComposedQueryBranchCarriesTheWorld(t *testing.T) {
	t.Parallel()
	spy := &worldCapturingSpy{}
	red := &countingRedactor{}
	// A composed ACL query: this is what an ACL-GATED principal produces,
	// and the branch the bug lived on.
	p := stubProvider{res: acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}}

	world := testWorld()
	_, ok := listPushdown(context.Background(), p, spy, red.redact,
		store.EntityQuery{Type: "ticket", World: world})
	if !ok {
		t.Fatal("pushdown declined a composable query")
	}
	if len(spy.graphQueries) != 1 {
		t.Fatalf("GraphQuery calls = %d, want 1", len(spy.graphQueries))
	}

	got := spy.graphQueries[0]
	if got.World.IsDefaultWorld() {
		t.Fatal("the composed GraphQuery reached the store with the DEFAULT world: " +
			"a world-scoped list silently degraded to unscoped for an ACL-gated " +
			"principal — drafts leak and `otherwise: exclude` stops excluding (RR-GQWRLD)")
	}
	res, scoped := got.World.For("ticket")
	if !scoped {
		t.Fatal("the world arrived but lost its ticket resolution")
	}
	if len(res.Chain) != 1 || res.Chain[0] != entity.Pointer("published") {
		t.Errorf("chain = %v, want [published]", res.Chain)
	}
	if res.Fallback != store.FallbackExclude {
		t.Errorf("fallback = %v, want exclude", res.Fallback)
	}
}

// TestWorldParity_AllowAllBranchCarriesTheWorld covers the other branch.
// AllowAll skips the GraphQuery entirely and lists directly, so it is a
// SEPARATE mechanism that could regress independently — which is the
// whole reason parity has to be asserted rather than assumed.
func TestWorldParity_AllowAllBranchCarriesTheWorld(t *testing.T) {
	t.Parallel()
	spy := &worldCapturingSpy{}
	red := &countingRedactor{}
	p := stubProvider{res: acl.ReadQueryResult{AllowAll: true}}

	world := testWorld()
	_, ok := listPushdown(context.Background(), p, spy, red.redact,
		store.EntityQuery{Type: "ticket", World: world})
	if !ok {
		t.Fatal("pushdown declined the AllowAll branch")
	}
	if len(spy.entityQueries) != 1 {
		t.Fatalf("ListEntities calls = %d, want 1", len(spy.entityQueries))
	}
	if spy.entityQueries[0].World.IsDefaultWorld() {
		t.Fatal("the AllowAll branch dropped the world")
	}
}

// TestWorldParity_BothBranchesAgree is the parity assertion proper: for
// the SAME requested world, the two branches must hand the store the same
// scope. A divergence here is the shape of the original bug — one path
// honoring the world while the other quietly serves the default.
func TestWorldParity_BothBranchesAgree(t *testing.T) {
	t.Parallel()
	world := testWorld()

	composedSpy := &worldCapturingSpy{}
	red := &countingRedactor{}
	composed := stubProvider{res: acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}}
	if _, ok := listPushdown(context.Background(), composed, composedSpy, red.redact,
		store.EntityQuery{Type: "ticket", World: world}); !ok {
		t.Fatal("composed branch declined")
	}

	allowSpy := &worldCapturingSpy{}
	allow := stubProvider{res: acl.ReadQueryResult{AllowAll: true}}
	if _, ok := listPushdown(context.Background(), allow, allowSpy, red.redact,
		store.EntityQuery{Type: "ticket", World: world}); !ok {
		t.Fatal("AllowAll branch declined")
	}

	gotComposed := composedSpy.graphQueries[0].World
	gotAllow := allowSpy.entityQueries[0].World

	// Compare the OBSERVABLE scope rather than the struct: WorldScope
	// holds an unexported map, and the contract is what For() answers.
	for _, typ := range []string{"ticket", "unscoped-type"} {
		cRes, cOK := gotComposed.For(typ)
		aRes, aOK := gotAllow.For(typ)
		if cOK != aOK {
			t.Fatalf("type %q: composed scoped=%v, AllowAll scoped=%v — the two "+
				"paths disagree about whether the world covers this type", typ, cOK, aOK)
		}
		if !cOK {
			continue
		}
		if len(cRes.Chain) != len(aRes.Chain) || cRes.Fallback != aRes.Fallback {
			t.Fatalf("type %q: composed=%v/%v, AllowAll=%v/%v — divergent resolution",
				typ, cRes.Chain, cRes.Fallback, aRes.Chain, aRes.Fallback)
		}
		for i := range cRes.Chain {
			if cRes.Chain[i] != aRes.Chain[i] {
				t.Fatalf("type %q: chain diverges at %d: %q vs %q",
					typ, i, cRes.Chain[i], aRes.Chain[i])
			}
		}
	}
}

// TestWorldParity_DefaultWorldStaysDefault pins the zero-cost path: a
// query carrying no world must not acquire one. The pointerless project
// has to keep paying nothing.
func TestWorldParity_DefaultWorldStaysDefault(t *testing.T) {
	t.Parallel()
	spy := &worldCapturingSpy{}
	red := &countingRedactor{}
	p := stubProvider{res: acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}}

	if _, ok := listPushdown(context.Background(), p, spy, red.redact,
		store.EntityQuery{Type: "ticket"}); !ok {
		t.Fatal("pushdown declined")
	}
	if !spy.graphQueries[0].World.IsDefaultWorld() {
		t.Error("a world-free query must reach the store world-free")
	}
}
