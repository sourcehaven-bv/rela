//go:build sqlite

package sqlitestore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// The SQL path for the simple graph shape must return exactly what
// graphquerynaive returns for the same query over the same data — rows AND
// order — including the values SQL and Go are most likely to read
// differently: numbers, booleans, JSON null, lists, quotes, non-ASCII text.
func TestGraphSQL_MatchesNaive(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	values := []any{
		"open", "done", "Open", "", nil, 7, 10, 2.5, true, false,
		[]any{"open", "x"}, `it's "quoted"`, "émigré", "zeta",
		map[string]any{"k": "v"}, 1e21, 1234567890123,
	}
	for i, v := range values {
		e := entity.New(fmt.Sprintf("T-%02d", i), "ticket")
		e.SetString("title", fmt.Sprintf("Ticket %02d", len(values)-i))
		if i%3 != 0 { // every third row has no `key` at all
			e.Properties["key"] = v
		}
		e.SetString("status", []string{"open", "done"}[i%2])
		e.Content = "body"
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	for _, id := range []string{"T-01", "T-04"} {
		e := entity.New(id, "ticket")
		e.Face = draft
		e.SetString("title", "draft of "+id)
		e.SetString("status", "open")
		e.SetString("key", "aaa-draft")
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	require.NoError(t, s.CreateEntity(ctx, entity.New("N-1", "note")))

	world := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {Chain: []entity.Face{draft}, Fallback: store.FallbackDefaultState},
	})
	eq := func(p, v string) store.PropPredicate {
		return store.PropPredicate{Property: p, Op: store.PropEqual, Value: v, Scalar: true}
	}
	queries := map[string]store.GraphQuery{
		"type only":         {EntityType: "ticket"},
		"eq":                {EntityType: "ticket", Props: []store.PropPredicate{eq("status", "open")}},
		"eq on mixed key":   {EntityType: "ticket", Props: []store.PropPredicate{eq("key", "open")}},
		"eq quoted value":   {EntityType: "ticket", Props: []store.PropPredicate{eq("key", `it's "quoted"`)}},
		"eq number text":    {EntityType: "ticket", Props: []store.PropPredicate{eq("key", "7")}},
		"order asc":         {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "key"}}},
		"order desc":        {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "key", Descending: true}}},
		"order two keys":    {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "status", Descending: true}, {Property: "title"}}},
		"page":              {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "title"}}, Limit: 4, Offset: 3},
		"offset only":       {EntityType: "ticket", Offset: 5},
		"limit only":        {EntityType: "ticket", Limit: 3},
		"world":             {EntityType: "ticket", World: world, OrderBy: []store.OrderSpec{{Property: "key"}}},
		"world eq on prime": {EntityType: "ticket", World: world, Props: []store.PropPredicate{eq("key", "aaa-draft")}},
		"face allowlist":    {EntityType: "ticket", World: world, FaceIn: []entity.Face{entity.Face("")}},
		"empty type":        {EntityType: "nothing"},
	}
	for name, q := range queries {
		t.Run(name, func(t *testing.T) {
			var want []string
			for e, err := range graphquerynaive.Run(ctx, s, q) {
				require.NoError(t, err)
				want = append(want, e.ID+"@"+string(e.Face))
			}
			var got, gotHeaders []string
			for e, err := range s.GraphQuery(ctx, q) {
				require.NoError(t, err)
				got = append(got, e.ID+"@"+string(e.Face))
				require.Equal(t, e.Content != "" || e.Face != "" || e.Type == "note", true, "rows keep their body")
			}
			for h, err := range store.GraphQueryHeaders(ctx, s, q) {
				require.NoError(t, err)
				gotHeaders = append(gotHeaders, h.ID+"@"+string(h.Face))
			}
			require.Equal(t, want, got, "rows")
			require.Equal(t, want, gotHeaders, "headers")

			wantCount, _, err := graphquerynaive.Count(ctx, s, q)
			require.NoError(t, err)
			gotCount, err := store.CountMatched(ctx, s, q)
			require.NoError(t, err)
			require.Equal(t, wantCount, gotCount, "count")
		})
	}
}

// With sort keys SQL renders exactly as Go does (text, integers, booleans,
// null, absent), the ordered read is served in SQL and must still equal the
// naive order. The mixed fixture above cannot show this: its floats, list and
// object make the exactness probe decline every ordering on `key`.
func TestGraphSQL_OrdersExactScalarsInSQL(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	for i, v := range []any{"open", "Open", "", nil, 7, 10, -3, true, false, "émigré", `q"uote`} {
		e := entity.New(fmt.Sprintf("T-%02d", i), "ticket")
		if i%4 != 0 {
			e.Properties["key"] = v
		}
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	for _, desc := range []bool{false, true} {
		q := store.GraphQuery{EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "key", Descending: desc}}, Limit: 8, Offset: 1}
		var want, got []string
		for e, err := range graphquerynaive.Run(ctx, s, q) {
			require.NoError(t, err)
			want = append(want, e.ID)
		}
		for h, err := range store.GraphQueryHeaders(ctx, s, q) {
			require.NoError(t, err)
			got = append(got, h.ID)
		}
		require.Equal(t, want, got, "descending=%v", desc)
	}
}
