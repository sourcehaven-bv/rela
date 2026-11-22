package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunGraphCondTests pins [store.GraphQuery.Where] and the typed ordered
// comparisons (TKT-8HY05WB): a boolean tree is two-valued on every backend,
// so NOT over a row without the property matches; a relation check may sit
// under OR and NOT; OrderString compares string values only, byte-wise;
// OrderInteger compares integers and integer-shaped strings only.
func RunGraphCondTests(t *testing.T, f Factory) {
	t.Helper()
	trivial := store.InWorld(store.TrivialScope())
	prop := func(name string, op store.PropOp, kind store.OrderKind, v string) store.Cond {
		return store.PropLeaf(store.PropPredicate{Property: name, Op: op, Value: v, Kind: kind})
	}
	eq := func(name, v string) store.Cond {
		return store.PropLeaf(store.PropPredicate{Property: name, Op: store.PropEqual, Value: v, Scalar: true})
	}
	where := func(t *testing.T, s store.Store, typ string, c store.Cond) []string {
		t.Helper()
		return runGraphQuery(t, s, store.GraphQuery{EntityType: typ, Where: &c, Faces: trivial})
	}

	seedTasks := func(t *testing.T, s store.Store) {
		t.Helper()
		seedEntityWithProps(t, s, "task", "T-1", map[string]any{"status": "open", "prio": 1, "name": "apple"})
		seedEntityWithProps(t, s, "task", "T-2", map[string]any{"status": "done", "prio": 5, "name": "Banana"})
		seedEntityWithProps(t, s, "task", "T-3", map[string]any{"status": "open", "prio": "7", "name": ""})
		seedEntityWithProps(t, s, "task", "T-4", map[string]any{"prio": 2.5, "name": []any{"x"}})
		seedEntityWithProps(t, s, "task", "T-5", map[string]any{"status": "", "prio": "+3", "name": 9})
		seedEntityWithProps(t, s, "task", "T-6", map[string]any{"prio": 1e19, "name": true})
		seedEntityWithProps(t, s, "task", "T-7", map[string]any{"prio": -4, "name": "zed"})
	}

	t.Run("Where_boolean_nodes", func(t *testing.T) {
		s := f(t)
		seedTasks(t, s)
		for _, tc := range []struct {
			name string
			cond store.Cond
			want []string
		}{
			{"or", store.OrCond(eq("status", "done"), eq("name", "apple")), []string{"T-1", "T-2"}},
			{"and", store.AndCond(eq("status", "open"), eq("name", "apple")), []string{"T-1"}},
			{"empty and is true", store.AndCond(),
				[]string{"T-1", "T-2", "T-3", "T-4", "T-5", "T-6", "T-7"}},
			{"empty or is false", store.OrCond(), nil},
			{"not over an absent key matches", store.NotCond(eq("status", "open")),
				[]string{"T-2", "T-4", "T-5", "T-6", "T-7"}},
			{"not over an ordered leaf matches the rows outside its range",
				store.NotCond(prop("prio", store.PropGreaterEqual, store.OrderInteger, "2")),
				[]string{"T-1", "T-4", "T-6", "T-7"}},
			{"double not", store.NotCond(store.NotCond(eq("status", "open"))), []string{"T-1", "T-3"}},
			{"not of or", store.NotCond(store.OrCond(eq("status", "open"), eq("status", "done"))),
				[]string{"T-4", "T-5", "T-6", "T-7"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.Equal(t, tc.want, where(t, s, "task", tc.cond))
			})
		}
	})

	t.Run("Where_ands_with_props", func(t *testing.T) {
		s := f(t)
		seedTasks(t, s)
		c := store.OrCond(eq("name", "apple"), eq("name", "Banana"))
		got := runGraphQuery(t, s, store.GraphQuery{
			EntityType: "task", Faces: trivial, Where: &c,
			Props: []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "done", Scalar: true}},
		})
		require.Equal(t, []string{"T-2"}, got)
	})

	t.Run("OrderInteger", func(t *testing.T) {
		s := f(t)
		seedTasks(t, s)
		for _, tc := range []struct {
			name string
			op   store.PropOp
			v    string
			want []string
		}{
			// T-3 holds "7" and T-5 "+3": integer-shaped strings compare as
			// integers. T-4 (2.5) and T-6 (1e19, out of int64) are not
			// integers and match no ordered comparison.
			{"greater", store.PropGreater, "2", []string{"T-2", "T-3", "T-5"}},
			{"greater equal", store.PropGreaterEqual, "5", []string{"T-2", "T-3"}},
			{"less", store.PropLess, "3", []string{"T-1", "T-7"}},
			{"less equal", store.PropLessEqual, "3", []string{"T-1", "T-5", "T-7"}},
			{"negative bound", store.PropGreater, "-5", []string{"T-1", "T-2", "T-3", "T-5", "T-7"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.Equal(t, tc.want, where(t, s, "task", prop("prio", tc.op, store.OrderInteger, tc.v)))
			})
		}
	})

	t.Run("OrderString", func(t *testing.T) {
		s := f(t)
		seedTasks(t, s)
		for _, tc := range []struct {
			name string
			op   store.PropOp
			v    string
			want []string
		}{
			// Byte-wise: "Banana" < "apple" < "zed". The empty string is a
			// string and takes part; numbers, booleans and lists do not.
			{"greater", store.PropGreater, "apple", []string{"T-7"}},
			{"greater equal", store.PropGreaterEqual, "apple", []string{"T-1", "T-7"}},
			{"less", store.PropLess, "apple", []string{"T-2", "T-3"}},
			{"less equal empty", store.PropLessEqual, "", []string{"T-3"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.Equal(t, tc.want, where(t, s, "task", prop("name", tc.op, store.OrderString, tc.v)))
			})
		}
	})

	t.Run("Where_related_under_or_and_not", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "ticket", "TKT-1", map[string]any{"status": "open"})
		seedEntityWithProps(t, s, "ticket", "TKT-2", map[string]any{"status": "done"})
		seedEntityWithProps(t, s, "ticket", "TKT-3", nil)
		seedGraphQueryEntities(t, s, "feature", "FEAT-1")
		mustRel(t, s, "TKT-1", "implements", "FEAT-1")
		mustRel(t, s, "TKT-3", "implements", "FEAT-1")
		impl := store.RelatedCond(store.DirectedRelation{Pred: store.RelationPredicate{OfTypes: []string{"implements"}}})
		require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3"},
			where(t, s, "ticket", store.OrCond(impl, eq("status", "done"))))
		require.Equal(t, []string{"TKT-2"}, where(t, s, "ticket", store.NotCond(impl)))
		require.Equal(t, []string{"TKT-3"},
			where(t, s, "ticket", store.AndCond(impl, store.NotCond(eq("status", "open")))))
	})

	// Where narrows the world's prime row, like Props: a family whose
	// published face matches but whose default row does not is found in a
	// world that prefers published, and not in the default world.
	t.Run("Where_tests_the_prime", func(t *testing.T) {
		s := f(t)
		published, err := entity.ParseFace("published")
		require.NoError(t, err)
		seedEntityWithProps(t, s, "item", "I-1", map[string]any{"prio": 1})
		fe := entity.New("I-1", "item")
		fe.Face = published
		fe.Properties = map[string]any{"prio": 9}
		require.NoError(t, s.CreateEntity(context.Background(), fe))
		c := store.OrCond(prop("prio", store.PropGreater, store.OrderInteger, "5"))
		world := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
			"item": {Chain: []entity.Face{"published"}, Fallback: store.FallbackDefaultState},
		}))
		require.Equal(t, []string{"I-1"},
			runGraphQuery(t, s, store.GraphQuery{EntityType: "item", Where: &c, Faces: world}))
		require.Empty(t, runGraphQuery(t, s, store.GraphQuery{EntityType: "item", Where: &c, Faces: trivial}))
	})

	// RowCond turns a query's row predicate into a Where (TKT-9BKVQ9I): a
	// reader ANDs a field grant's condition onto a page filter this way, so
	// the Where must match exactly the rows the query does. The relation
	// predicates carry the shapes a role grant uses: group expansion,
	// containment inheritance, edge properties, an endpoint match and
	// negation.
	t.Run("RowCond_matches_the_query", func(t *testing.T) {
		s := f(t)
		seedEntityWithProps(t, s, "doc", "D-1", map[string]any{"status": "open"})
		seedEntityWithProps(t, s, "doc", "D-2", map[string]any{"status": "done"})
		seedEntityWithProps(t, s, "doc", "D-3", map[string]any{"status": "open"})
		seedEntityWithProps(t, s, "doc", "D-4", nil)
		seedGraphQueryEntities(t, s, "folder", "F-1")
		seedGraphQueryEntities(t, s, "user", "U-1", "U-2")
		seedGraphQueryEntities(t, s, "group", "G-1")
		mustRel(t, s, "U-1", "member-of", "G-1")
		mustRelProps(t, s, "G-1", "owns", "D-1", map[string]any{"active": "yes"})
		mustRelProps(t, s, "U-2", "owns", "D-2", map[string]any{"active": "no"})
		mustRel(t, s, "D-3", "belongs-to", "F-1")
		mustRel(t, s, "U-1", "owns", "F-1")
		owns := store.RelationPredicate{
			Endpoints: []string{"U-1"}, OfTypes: []string{"owns"},
			InheritThrough: []string{"member-of"}, Depth: 3,
			EntityInheritThrough: []string{"belongs-to"}, EntityDepth: 3,
		}
		active := owns
		active.EdgeProps = []store.PropPredicate{{Property: "active", Op: store.PropEqual, Value: "yes", Scalar: true}}
		notOwned := owns
		notOwned.Negate = true
		byGroup := store.RelationPredicate{
			OfTypes: []string{"owns"}, EndpointMatch: &store.EndpointPredicate{EntityType: "group", Faces: store.AllFaces()},
		}
		open := store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}
		notDone := store.NotCond(store.PropLeaf(
			store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "done", Scalar: true}))
		for _, tc := range []struct {
			name string
			q    store.GraphQuery
		}{
			{"props", store.GraphQuery{Props: []store.PropPredicate{open}}},
			{"related with expansions", store.GraphQuery{Related: []store.DirectedRelation{{Incoming: true, Pred: owns}}}},
			{"edge props", store.GraphQuery{Related: []store.DirectedRelation{{Incoming: true, Pred: active}}}},
			{"negated related", store.GraphQuery{Related: []store.DirectedRelation{{Incoming: true, Pred: notOwned}}}},
			{"endpoint match", store.GraphQuery{Related: []store.DirectedRelation{{Incoming: true, Pred: byGroup}}}},
			{"props, related and where", store.GraphQuery{
				Props:   []store.PropPredicate{open},
				Related: []store.DirectedRelation{{Incoming: true, Pred: owns}},
				Where:   &notDone,
			}},
			{"empty", store.GraphQuery{}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				q := tc.q
				q.EntityType, q.Faces = "doc", trivial
				c, err := q.RowCond()
				require.NoError(t, err)
				want := runGraphQuery(t, s, q)
				require.NotEmpty(t, want, "a case that matches nothing proves nothing")
				require.Equal(t, want, where(t, s, "doc", c))
			})
		}
	})

	t.Run("Where_invalid_is_refused", func(t *testing.T) {
		s := f(t)
		seedTasks(t, s)
		deep := eq("status", "open")
		for range store.MaxCondDepth + 1 {
			deep = store.NotCond(deep)
		}
		for _, tc := range []struct {
			name string
			cond store.Cond
		}{
			{"integer kind needs an integer value", prop("prio", store.PropGreater, store.OrderInteger, "2.5")},
			{"kind needs an ordered operator", store.PropLeaf(store.PropPredicate{
				Property: "prio", Op: store.PropEqual, Value: "1", Kind: store.OrderInteger})},
			{"not takes one argument", store.Cond{Op: store.CondNot}},
			{"too deep", deep},
			{"negate inside a tree", store.RelatedCond(store.DirectedRelation{
				Pred: store.RelationPredicate{OfTypes: []string{"implements"}, Negate: true}})},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := tc.cond
				var gotErr error
				for _, err := range s.GraphQuery(context.Background(),
					store.GraphQuery{EntityType: "task", Where: &c, Faces: trivial}) {
					if err != nil {
						gotErr = err
						break
					}
				}
				require.True(t, errors.Is(gotErr, store.ErrInvalidQuery), "got %v", gotErr)
			})
		}
	})
}
