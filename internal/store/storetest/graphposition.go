package storetest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunGraphPositionTests pins [store.GraphPosition] to the one definition it
// can have: for every matched id, the position equals what the ordered
// GraphQuery result says (TKT-U9DYW4). Stating it as a differential rather
// than as expected literals is the point — a backend answers a position
// natively (pgstore, one window-function statement) precisely where it also
// orders natively, and the two must not drift.
func RunGraphPositionTests(t *testing.T, f Factory) {
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		dues := []string{"2026-03-01", "", "2026-01-15", "2026-03-01", "", "2025-12-31", "null"}
		for i, due := range dues {
			e := entity.New(fmt.Sprintf("T-%d", i), "ticket")
			e.SetString("status", []string{"open", "done"}[i%2])
			switch due {
			case "":
			case "null":
				e.Properties["due"] = nil
			default:
				e.SetString("due", due)
			}
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		// Faced rows, so a world query resolves primes: T-0 has a draft
		// state, T-2 exists only as a draft state beside its default.
		for _, id := range []string{"T-0", "T-2"} {
			e := entity.New(id, "ticket")
			face, err := entity.ParseFace("draft")
			require.NoError(t, err)
			e.Face = face
			e.SetString("status", "open")
			e.SetString("due", "2024-01-01")
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		other := entity.New("N-1", "note")
		require.NoError(t, s.CreateEntity(ctx(), other))
	}
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	draftWorld := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {Chain: []entity.Face{draft}, Fallback: store.FallbackDefaultState},
	})
	draftOnly := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {Chain: []entity.Face{draft}, Fallback: store.FallbackExclude},
	})

	queries := map[string]store.GraphQuery{
		"id order":   {EntityType: "ticket"},
		"due asc":    {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "due"}}},
		"due desc":   {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "due", Descending: true}}},
		"two keys":   {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "status", Descending: true}, {Property: "due"}}},
		"filtered":   {EntityType: "ticket", Props: []store.PropPredicate{{Property: "status", Value: "open"}}, OrderBy: []store.OrderSpec{{Property: "due"}}},
		"paged":      {EntityType: "ticket", OrderBy: []store.OrderSpec{{Property: "due"}}, Limit: 2, Offset: 3},
		"world":      {EntityType: "ticket", World: draftWorld, OrderBy: []store.OrderSpec{{Property: "due"}}},
		"world only": {EntityType: "ticket", World: draftOnly},
		"single row": {EntityType: "note"},
	}

	for name, q := range queries {
		t.Run(name, func(t *testing.T) {
			s := f(t)
			seed(t, s)

			unpaged := q
			unpaged.Limit, unpaged.Offset = 0, 0
			var ordered []store.EntityHeader
			for h, err := range store.GraphQueryHeaders(ctx(), s, unpaged) {
				require.NoError(t, err)
				ordered = append(ordered, h)
			}
			require.NotEmpty(t, ordered)

			for i, h := range ordered {
				pos, found, err := store.GraphPosition(ctx(), s, q, h.ID)
				require.NoError(t, err)
				require.True(t, found, "id %s", h.ID)
				want := store.Position{Index: i + 1, Total: len(ordered)}
				if i > 0 {
					want.Prev = &store.EntityRef{ID: ordered[i-1].ID, Type: ordered[i-1].Type}
				}
				if i < len(ordered)-1 {
					want.Next = &store.EntityRef{ID: ordered[i+1].ID, Type: ordered[i+1].Type}
				}
				require.Equal(t, want, pos, "id %s", h.ID)
			}

			_, found, err := store.GraphPosition(ctx(), s, q, "T-404")
			require.NoError(t, err)
			require.False(t, found)
		})
	}

	t.Run("empty result", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		q := store.GraphQuery{EntityType: "ticket", Props: []store.PropPredicate{{Property: "status", Value: "no-such-status"}}}
		_, found, err := store.GraphPosition(ctx(), s, q, "T-0")
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("unmatched id of the type is not found", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		q := store.GraphQuery{EntityType: "ticket", Props: []store.PropPredicate{{Property: "status", Value: "open"}}}
		_, found, err := store.GraphPosition(ctx(), s, q, "T-1") // status done
		require.NoError(t, err)
		require.False(t, found)
	})
}
