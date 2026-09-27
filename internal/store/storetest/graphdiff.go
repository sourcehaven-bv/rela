package storetest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// graphDiffQueries is how many generated queries RunGraphDifferential checks.
const graphDiffQueries = 400

// RunGraphDifferential compares a backend's native graph queries with
// graphquerynaive, the behavioral reference, on ONE seeded store. The seed
// mixes every value shape a property can hold (strings, the empty string,
// JSON null, absent keys, lists, the empty list, integers, booleans, a real),
// awkward property names, faces, state-tailed edges, inheritance chains with
// a cycle, and dangling-free endpoints; the queries are generated from fixed
// seeds so a failure reproduces.
//
// The reference runs graphquerynaive over a memstore seeded identically, not
// over the store under test: graphquerynaive reads through ListEntities, and a
// SQL backend's ListEntities shares its world ranking with its graph queries,
// so a ranking defect would otherwise agree with itself. A backend that
// delegates to graphquerynaive exercises only its ListEntities here.
//
// Property names a SQL backend cannot render send the whole query to its
// naive fallback, so the generator uses one rarely: most queries must take
// the SQL path to test anything.
//
// Two answers are deliberately NOT compared, because they are documented
// divergences rather than defects: the GraphCount total under a world with
// Any (BUG-OQQE8D), and sort keys over list or real values, which Go and SQL
// render differently (the generator never orders by those properties).
func RunGraphDifferential(t *testing.T, f Factory) {
	s := f(t)
	seedGraphDiff(t, s)
	ref := memstore.New()
	seedGraphDiff(t, ref)
	c := context.Background()
	ids := []string{"GRP-1", "NOPE-1"}
	for i := range graphDiffItems {
		ids = append(ids, fmt.Sprintf("ITM-%02d", i))
	}

	fails := 0
	for i := range graphDiffQueries {
		rng := rand.New(rand.NewPCG(uint64(i), 0xB51C))
		q := genGraphQuery(rng)
		if msg := diffGraphQuery(c, ref, s, q, ids); msg != "" {
			fails++
			t.Errorf("query %d: %s\nquery: %+v", i, msg, q)
			if fails >= 100 {
				t.Fatal("too many differences; stopping")
			}
		}
	}
}

// diffGraphQuery returns a description of the first disagreement between s
// and graphquerynaive on q, or "".
func diffGraphQuery(c context.Context, ref, s store.Store, q store.GraphQuery, ids []string) string {
	want, wantErr := drainEntities(graphquerynaive.Run(c, ref, q))
	got, gotErr := drainEntities(s.GraphQuery(c, q))
	if (wantErr == nil) != (gotErr == nil) {
		return fmt.Sprintf("GraphQuery error: naive %v, store %v", wantErr, gotErr)
	}
	if wantErr != nil {
		return ""
	}
	if w, g := entityKeys(want), entityKeys(got); !slices.Equal(w, g) {
		return fmt.Sprintf("GraphQuery rows: naive %v, store %v", w, g)
	}
	for i := range want {
		if !propsEqual(want[i].Properties, got[i].Properties) {
			return fmt.Sprintf("GraphQuery %s properties: naive %v, store %v",
				want[i].ID, want[i].Properties, got[i].Properties)
		}
	}

	var heads []string
	for h, err := range store.GraphQueryHeaders(c, s, q) {
		if err != nil {
			return "GraphQueryHeaders: " + err.Error()
		}
		heads = append(heads, h.ID+"@"+h.Face.String())
	}
	if w := entityKeys(want); !slices.Equal(w, heads) {
		return fmt.Sprintf("GraphQueryHeaders: naive %v, store %v", w, heads)
	}

	unpaged := q
	unpaged.Limit, unpaged.Offset, unpaged.OrderBy = 0, 0, nil
	wantMatched, wantTotal, err := graphquerynaive.Count(c, ref, unpaged)
	if err != nil {
		return "naive Count: " + err.Error()
	}
	gotMatched, gotTotal, err := s.GraphCount(c, unpaged)
	if err != nil {
		return "GraphCount: " + err.Error()
	}
	if wantMatched != gotMatched {
		return fmt.Sprintf("GraphCount matched: naive %d, store %d", wantMatched, gotMatched)
	}
	knownTotalDivergence := len(q.Any) > 0 && !q.World.IsDefaultWorld() // BUG-OQQE8D
	if wantTotal != gotTotal && !knownTotalDivergence {
		return fmt.Sprintf("GraphCount total: naive %d, store %d", wantTotal, gotTotal)
	}
	if n, cerr := store.CountMatched(c, s, unpaged); cerr != nil || n != wantMatched {
		return fmt.Sprintf("CountMatched: naive %d, store %d (%v)", wantMatched, n, cerr)
	}

	wantIDs, err := graphquerynaive.MatchingIDs(c, ref, unpaged, ids)
	if err != nil {
		return "naive MatchingIDs: " + err.Error()
	}
	gotIDs, err := s.MatchingIDs(c, unpaged, ids)
	if err != nil {
		return "MatchingIDs: " + err.Error()
	}
	for _, id := range ids {
		if wantIDs[id] != gotIDs[id] {
			return fmt.Sprintf("MatchingIDs[%s]: naive %v, store %v", id, wantIDs[id], gotIDs[id])
		}
	}
	return ""
}

func drainEntities(seq func(func(*entity.Entity, error) bool)) ([]*entity.Entity, error) {
	var out []*entity.Entity
	for e, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func entityKeys(es []*entity.Entity) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.ID + "@" + e.Face.String()
	}
	return out
}

func propsEqual(a, b map[string]any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// graphDiffItems is how many candidate entities the seed creates.
const graphDiffItems = 24

// graphDiffProps are the property names the generator filters on. They
// include names a SQL backend must quote inside a JSON path.
var graphDiffProps = []string{"status", "title", "prio", "flag", "it's", "a.b", "sp ace", "ünï"}

// graphDiffUnsafeProp is a name no SQL backend renders as a literal JSON path.
// The generator picks it rarely; see RunGraphDifferential.
const graphDiffUnsafeProp = `quo"te`

// graphDiffOrderProps are the properties the generator sorts by: never a list
// or a real, whose text forms differ between Go and SQL.
var graphDiffOrderProps = []string{"title", "prio", "flag", "it's"}

// graphDiffValues is the value mix, cycled over the items per property.
var graphDiffValues = []any{
	"open", "closed", "", nil, "absent", []any{"open", "x"}, []any{}, 5, true, 1.5,
	"alpha", "Zed", "ñandú", false, 10, 9, []any{"closed"}, "x",
}

// graphDiffScalars is the value mix for the sort properties: no lists and no
// reals.
var graphDiffScalars = []any{"open", "", nil, "absent", 5, true, "alpha", "Zed", "ñandú", false, 10, 9, "x", "it's"}

// graphDiffTargets are the comparison values the generator picks from.
var graphDiffTargets = []string{"open", "closed", "", "5", "true", "1.5", "x", "alpha", "Zed", "10", "9", "false", "m"}

func seedGraphDiff(t *testing.T, s store.Store) {
	t.Helper()
	c := context.Background()
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	published, err := entity.ParseFace("published")
	require.NoError(t, err)

	for i := 1; i <= 4; i++ {
		g := entity.New(fmt.Sprintf("GRP-%d", i), "group")
		g.Properties = map[string]any{"status": []string{"open", "closed", "open", ""}[i-1], "title": fmt.Sprintf("g%d", i)}
		require.NoError(t, s.CreateEntity(c, g))
	}
	for i := range graphDiffItems {
		props := map[string]any{}
		for j, p := range append(slices.Clone(graphDiffProps), graphDiffUnsafeProp) {
			mix := graphDiffValues
			if slices.Contains(graphDiffOrderProps, p) {
				mix = graphDiffScalars
			}
			v := mix[(i*(j+2)+j)%len(mix)]
			if v == "absent" {
				continue
			}
			props[p] = v
		}
		e := entity.New(fmt.Sprintf("ITM-%02d", i), "item")
		e.Properties = props
		require.NoError(t, s.CreateEntity(c, e))
		for k, face := range []entity.Face{draft, published} {
			if (i+k)%3 != 0 {
				continue
			}
			fe := entity.New(e.ID, "item")
			fe.Face = face
			fe.Properties = map[string]any{
				"status": []string{"open", "closed"}[(i+k)%2],
				"title":  fmt.Sprintf("%s-%s", face, e.ID),
				"prio":   i % 4,
			}
			require.NoError(t, s.CreateEntity(c, fe))
		}
	}

	rel := func(from, typ, to string, face entity.Face) {
		_, err := s.CreateRelation(c, from, typ, to, &store.RelationData{FromFace: face})
		require.NoError(t, err)
	}
	// Groups form a chain with a cycle, for the endpoint closure.
	rel("GRP-1", "partOf", "GRP-2", "")
	rel("GRP-2", "partOf", "GRP-3", "")
	rel("GRP-3", "partOf", "GRP-1", "")
	rel("GRP-3", "partOf", "GRP-4", "")
	for i := range graphDiffItems {
		id := fmt.Sprintf("ITM-%02d", i)
		if i%3 == 0 {
			rel(id, "implements", fmt.Sprintf("GRP-%d", i%4+1), "")
		}
		if i%5 == 1 {
			rel(id, "tracks", fmt.Sprintf("GRP-%d", (i+1)%4+1), "")
		}
		if i%4 == 2 {
			// Items form parent chains, for the entity closure.
			rel(id, "childOf", fmt.Sprintf("ITM-%02d", (i+5)%graphDiffItems), "")
		}
		if i%7 == 3 && i%3 == 0 {
			rel(id, "implements", "GRP-4", draft)
		}
		if i%6 == 5 {
			rel(fmt.Sprintf("GRP-%d", i%4+1), "owns", id, "")
		}
	}
}

func pick[T any](rng *rand.Rand, xs []T) T { return xs[rng.IntN(len(xs))] }

func pickSome[T any](rng *rand.Rand, xs []T, most int) []T {
	n := rng.IntN(most + 1)
	out := make([]T, 0, n)
	for range n {
		out = append(out, pick(rng, xs))
	}
	return out
}

func genProp(rng *rand.Rand) store.PropPredicate {
	ops := []store.PropOp{store.PropEqual, store.PropNotEqual, store.PropNotEqualOrEmpty,
		store.PropGreaterEqual, store.PropLessEqual}
	prop := pick(rng, graphDiffProps)
	if rng.IntN(40) == 0 {
		prop = graphDiffUnsafeProp
	}
	p := store.PropPredicate{Property: prop, Op: pick(rng, ops), Value: pick(rng, graphDiffTargets)}
	p.Scalar = p.Op == store.PropEqual && rng.IntN(2) == 0
	return p
}

func genRelation(rng *rand.Rand, nested bool) store.RelationPredicate {
	p := store.RelationPredicate{
		OfTypes:   pickSome(rng, []string{"implements", "tracks", "owns", "childOf"}, 2),
		Endpoints: pickSome(rng, []string{"GRP-1", "GRP-2", "GRP-3", "GRP-4", "ITM-07", "NOPE-1"}, 2),
		Negate:    rng.IntN(4) == 0,
	}
	if !nested && rng.IntN(3) == 0 {
		// Up to past graphquerynaive.DepthCap, which both paths must apply.
		p.InheritThrough, p.Depth = []string{"partOf"}, rng.IntN(8)
	}
	if !nested && rng.IntN(4) == 0 {
		p.EntityInheritThrough, p.EntityDepth = []string{"childOf"}, rng.IntN(8)
	}
	if rng.IntN(3) == 0 {
		m := &store.EndpointPredicate{EntityType: pick(rng, []string{"", "group", "item"})}
		for range rng.IntN(2) {
			m.Props = append(m.Props, genProp(rng))
		}
		if !nested && rng.IntN(3) == 0 {
			hop := genRelation(rng, true)
			m.HasOutbound = &hop
		}
		p.EndpointMatch = m
	}
	return p
}

func genGraphQuery(rng *rand.Rand) store.GraphQuery {
	q := store.GraphQuery{EntityType: "item"}
	for range rng.IntN(3) {
		q.Props = append(q.Props, genProp(rng))
	}
	if rng.IntN(3) == 0 {
		p := genRelation(rng, false)
		q.HasInbound = &p
	}
	if rng.IntN(3) == 0 {
		p := genRelation(rng, false)
		q.HasOutbound = &p
	}
	for range rng.IntN(2) {
		q.Related = append(q.Related, store.DirectedRelation{Pred: genRelation(rng, false), Incoming: rng.IntN(2) == 0})
	}
	faces := []entity.Face{"", "draft", "published"}
	if rng.IntN(4) == 0 {
		for range rng.IntN(3) + 1 {
			br := store.GraphBranch{FaceIn: pickSome(rng, faces, 2)}
			if rng.IntN(2) == 0 {
				p := genRelation(rng, false)
				br.HasInbound = &p
			}
			q.Any = append(q.Any, br)
		}
	}
	if rng.IntN(4) == 0 {
		for range rng.IntN(2) + 1 {
			q.Narrowing = append(q.Narrowing, store.NarrowBranch{Props: []store.PropPredicate{genProp(rng)}})
		}
	}
	switch rng.IntN(4) {
	case 1:
		q.World = store.NewWorldScope(map[string]store.TypeResolution{
			"item": {Chain: []entity.Face{"published"}, Fallback: store.FallbackDefaultState},
		})
	case 2:
		q.World = store.NewWorldScope(map[string]store.TypeResolution{
			"item": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
		})
	case 3:
		// A world that does not scope the queried type collapses to default.
		q.World = store.NewWorldScope(map[string]store.TypeResolution{
			"group": {Chain: []entity.Face{"draft"}, Fallback: store.FallbackExclude},
		})
	}
	if rng.IntN(5) == 0 {
		q.FaceIn = pickSome(rng, faces, 2)
	}
	for range rng.IntN(3) {
		spec := store.OrderSpec{Property: pick(rng, graphDiffOrderProps), Descending: rng.IntN(2) == 0}
		if rng.IntN(3) == 0 {
			spec.Values = []string{"Zed", "alpha", "5", "true", "it's"}
		}
		q.OrderBy = append(q.OrderBy, spec)
	}
	if rng.IntN(3) == 0 {
		q.Limit = rng.IntN(6)
	}
	if rng.IntN(3) == 0 {
		q.Offset = rng.IntN(8)
	}
	return q
}
