package storeutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

func ptr(t *testing.T, v string) entity.Pointer {
	t.Helper()
	p, err := entity.ParsePointer(v)
	require.NoError(t, err)
	return p
}

// pageScope builds a one-type world over `page`.
func pageScope(t *testing.T, fb store.Fallback, chain ...string) store.WorldScope {
	t.Helper()
	coords := make([]entity.Pointer, 0, len(chain))
	for _, c := range chain {
		coords = append(coords, ptr(t, c))
	}
	return store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: coords, Fallback: fb},
	})
}

func cand(id, typ string, p entity.Pointer) storeutil.WorldCandidate {
	return storeutil.WorldCandidate{ID: id, Type: typ, Pointer: p}
}

// TestCompareStateKeys pins the ordering the whole world path depends on:
// state keys sort as the TUPLE (bare id, pointer), not as the joined
// string. '@' is 0x40 and the digits are 0x30-0x39, so under plain string
// order a numerically-prefixed sibling lands inside the family.
func TestCompareStateKeys(t *testing.T) {
	t.Run("a family is contiguous", func(t *testing.T) {
		keys := []string{"PAGE-2", "PAGE-1@published", "PAGE-10", "PAGE-1", "PAGE-10@draft", "PAGE-1@draft"}
		var got []string
		for _, k := range keys {
			got = storeutil.SortedInsertFunc(got, k, storeutil.CompareStateKeys)
		}
		assert.Equal(t, []string{
			"PAGE-1", "PAGE-1@draft", "PAGE-1@published", "PAGE-10", "PAGE-10@draft", "PAGE-2",
		}, got, "PAGE-10 must not sort between PAGE-1 and its states")
	})

	t.Run("ordering is total and self-consistent", func(t *testing.T) {
		cases := []struct {
			a, b string
			want int
		}{
			{"PAGE-1", "PAGE-1", 0},
			{"PAGE-1", "PAGE-1@draft", -1},  // default state sorts first
			{"PAGE-1@draft", "PAGE-1", 1},   // and the inverse
			{"PAGE-1@draft", "PAGE-10", -1}, // the whole point
			{"PAGE-1@draft", "PAGE-1@published", -1},
			{"PAGE-2", "PAGE-10", 1}, // lexical on the id, as before
		}
		for _, tc := range cases {
			assert.Equal(t, tc.want, storeutil.CompareStateKeys(tc.a, tc.b), "%s vs %s", tc.a, tc.b)
		}
	})
}

func TestSortedInsertRemoveFunc(t *testing.T) {
	var keys []string
	for _, k := range []string{"PAGE-10", "PAGE-1@draft", "PAGE-1"} {
		keys = storeutil.SortedInsertFunc(keys, k, storeutil.CompareStateKeys)
	}
	assert.Equal(t, []string{"PAGE-1", "PAGE-1@draft", "PAGE-10"}, keys)

	keys = storeutil.SortedRemoveFunc(keys, "PAGE-1@draft", storeutil.CompareStateKeys)
	assert.Equal(t, []string{"PAGE-1", "PAGE-10"}, keys)

	assert.Panics(t, func() {
		storeutil.SortedRemoveFunc(keys, "PAGE-404", storeutil.CompareStateKeys)
	}, "removing a missing key is a caller bug, not a silent no-op")
}

// TestWorldPrimes covers the three resolution rules directly, so a
// regression is attributed here rather than only surfacing through a
// backend's conformance run.
func TestWorldPrimes(t *testing.T) {
	def := entity.Pointer("")

	t.Run("rule 2: first existing coordinate wins", func(t *testing.T) {
		got := storeutil.WorldPrimes(
			pageScope(t, store.FallbackExclude, "review", "published"),
			[]storeutil.WorldCandidate{
				cand("PAGE-1", "page", def),
				cand("PAGE-1", "page", ptr(t, "review")),
				cand("PAGE-1", "page", ptr(t, "published")),
				cand("PAGE-2", "page", def),
				cand("PAGE-2", "page", ptr(t, "published")),
			})
		assert.Equal(t, map[string]entity.Pointer{
			"PAGE-1": ptr(t, "review"),
			"PAGE-2": ptr(t, "published"),
		}, got)
	})

	t.Run("rule 2 is order-sensitive, not set membership", func(t *testing.T) {
		cands := []storeutil.WorldCandidate{
			cand("PAGE-1", "page", ptr(t, "review")),
			cand("PAGE-1", "page", ptr(t, "published")),
		}
		fwd := storeutil.WorldPrimes(pageScope(t, store.FallbackExclude, "review", "published"), cands)
		rev := storeutil.WorldPrimes(pageScope(t, store.FallbackExclude, "published", "review"), cands)
		assert.Equal(t, ptr(t, "review"), fwd["PAGE-1"])
		assert.Equal(t, ptr(t, "published"), rev["PAGE-1"])
	})

	t.Run("candidate order must not change the answer", func(t *testing.T) {
		// The chain decides, not the order rows happen to arrive in.
		w := pageScope(t, store.FallbackExclude, "review", "published")
		forward := storeutil.WorldPrimes(w, []storeutil.WorldCandidate{
			cand("PAGE-1", "page", ptr(t, "review")),
			cand("PAGE-1", "page", ptr(t, "published")),
		})
		backward := storeutil.WorldPrimes(w, []storeutil.WorldCandidate{
			cand("PAGE-1", "page", ptr(t, "published")),
			cand("PAGE-1", "page", ptr(t, "review")),
		})
		assert.Equal(t, forward, backward)
		assert.Equal(t, ptr(t, "review"), forward["PAGE-1"])
	})

	t.Run("rule 3: exclude contributes nothing", func(t *testing.T) {
		got := storeutil.WorldPrimes(
			pageScope(t, store.FallbackExclude, "published"),
			[]storeutil.WorldCandidate{cand("PAGE-2", "page", def)})
		assert.NotContains(t, got, "PAGE-2",
			"absence IS the publication bit — no fallback to the draft face")
	})

	t.Run("rule 3: default falls back to the default state", func(t *testing.T) {
		got := storeutil.WorldPrimes(
			pageScope(t, store.FallbackDefaultState, "published"),
			[]storeutil.WorldCandidate{cand("PAGE-2", "page", def)})
		assert.Equal(t, map[string]entity.Pointer{"PAGE-2": def}, got)
	})

	t.Run("rule 3 default cannot invent a missing default row", func(t *testing.T) {
		// A headless family (no default row) has nothing to fall back to.
		got := storeutil.WorldPrimes(
			pageScope(t, store.FallbackDefaultState, "published"),
			[]storeutil.WorldCandidate{cand("PAGE-3", "page", ptr(t, "archived"))})
		assert.NotContains(t, got, "PAGE-3")
	})

	t.Run("rule 1: a type the world does not name keeps its default state", func(t *testing.T) {
		got := storeutil.WorldPrimes(
			pageScope(t, store.FallbackExclude, "published"),
			[]storeutil.WorldCandidate{
				cand("TKT-1", "ticket", def),
				cand("PAGE-1", "page", def),
				cand("PAGE-1", "page", ptr(t, "published")),
			})
		assert.Equal(t, def, got["TKT-1"],
			"absence from the scope is rule 1, NOT the zero TypeResolution's exclude")
		assert.Equal(t, ptr(t, "published"), got["PAGE-1"])
	})

	t.Run("no candidates", func(t *testing.T) {
		assert.Nil(t, storeutil.WorldPrimes(pageScope(t, store.FallbackExclude, "published"), nil))
	})
}

// TestPaginateWorldPrimes pins that the limit counts PRIMES rather than
// candidate rows, and that paging never splits or duplicates a family.
func TestPaginateWorldPrimes(t *testing.T) {
	// Three entities, each with a default and a published face, in the
	// tuple order the fs/mem index maintains.
	keys := []string{
		"PAGE-1", "PAGE-1@published",
		"PAGE-10", "PAGE-10@published",
		"PAGE-2", "PAGE-2@published",
	}
	load := func(key string) (storeutil.WorldCandidate, bool) {
		id, p, err := entity.ParseStateRef(key)
		if err != nil {
			return storeutil.WorldCandidate{}, false
		}
		return storeutil.WorldCandidate{ID: id, Type: "page", Pointer: p}, true
	}
	always := func(string) bool { return true }
	w := pageScope(t, store.FallbackDefaultState, "published")

	t.Run("the limit counts primes, not rows", func(t *testing.T) {
		// Six rows, three primes. A limit of 2 must yield 2 entities,
		// not 2 rows of one entity's family.
		page := storeutil.PaginateWorldPrimes(keys, "", 2, w, always, load)
		assert.Equal(t, []string{"PAGE-1@published", "PAGE-10@published"}, page.Keys)
		assert.NotEmpty(t, page.NextCursor, "a further prime exists")
	})

	t.Run("paging visits every prime exactly once", func(t *testing.T) {
		for _, limit := range []int{1, 2, 3, 5} {
			var got []string
			cursor := ""
			for range 10 {
				page := storeutil.PaginateWorldPrimes(keys, cursor, limit, w, always, load)
				got = append(got, page.Keys...)
				if page.NextCursor == "" {
					break
				}
				decoded, err := storeutil.DecodeCursor(page.NextCursor)
				require.NoError(t, err)
				cursor = decoded
			}
			assert.Equal(t, []string{
				"PAGE-1@published", "PAGE-10@published", "PAGE-2@published",
			}, got, "limit %d", limit)
		}
	})

	t.Run("an unmatched family still closes cleanly", func(t *testing.T) {
		// Filtering out PAGE-10 entirely must not merge its neighbors.
		match := func(key string) bool {
			id, _, _ := entity.ParseStateRef(key)
			return id != "PAGE-10"
		}
		page := storeutil.PaginateWorldPrimes(keys, "", 0, w, match, load)
		assert.Equal(t, []string{"PAGE-1@published", "PAGE-2@published"}, page.Keys)
	})

	t.Run("a row that vanished is skipped", func(t *testing.T) {
		gone := func(key string) (storeutil.WorldCandidate, bool) {
			if key == "PAGE-10@published" {
				return storeutil.WorldCandidate{}, false
			}
			return load(key)
		}
		page := storeutil.PaginateWorldPrimes(keys, "", 0, w, always, gone)
		// PAGE-10 keeps only its default row, so the fallback supplies it.
		assert.Equal(t, []string{"PAGE-1@published", "PAGE-10", "PAGE-2@published"}, page.Keys)
	})
}

// TestValidateEntityQuery pins decision Q3: AllStates is raw storage
// truth and a world resolves each entity to one state, so a query
// setting both is refused rather than silently resolved.
func TestValidateEntityQuery(t *testing.T) {
	world := pageScope(t, store.FallbackExclude, "published")

	t.Run("the contradiction is rejected", func(t *testing.T) {
		err := storeutil.ValidateEntityQuery(store.EntityQuery{AllStates: true, World: world})
		assert.ErrorIs(t, err, store.ErrInvalidQuery)
	})

	t.Run("either alone is fine", func(t *testing.T) {
		// The zero WorldScope is the DEFAULT world, not "no world" — a
		// check keyed on the wrong condition would reject every existing
		// query in the codebase.
		for _, q := range []store.EntityQuery{
			{},
			{AllStates: true},
			{World: world},
			{World: store.DefaultWorld()},
			{AllStates: true, World: store.DefaultWorld()},
		} {
			assert.NoError(t, storeutil.ValidateEntityQuery(q), "%+v", q)
		}
	})
}

// TestMatchEntityQuery pins the shared filter, including the widening
// that world resolution depends on.
func TestMatchEntityQuery(t *testing.T) {
	draft := ptr(t, "draft")
	def := entity.Pointer("")

	t.Run("default-only unless AllStates or a world", func(t *testing.T) {
		assert.True(t, storeutil.MatchEntityQuery("page", "PAGE-1", def, store.EntityQuery{}, nil))
		assert.False(t, storeutil.MatchEntityQuery("page", "PAGE-1", draft, store.EntityQuery{}, nil),
			"a state row is not in the default world")
		assert.True(t, storeutil.MatchEntityQuery(
			"page", "PAGE-1", draft, store.EntityQuery{AllStates: true}, nil))
	})

	t.Run("a non-default world WIDENS the pointer filter", func(t *testing.T) {
		// Resolution picks the prime afterwards, so every state must
		// reach it. Filtering to default rows first would leave a world
		// able to return only default faces — and that reads as a
		// correctly-empty published world rather than as an error.
		q := store.EntityQuery{World: pageScope(t, store.FallbackExclude, "published")}
		assert.True(t, storeutil.MatchEntityQuery("page", "PAGE-1", draft, q, nil))
		assert.True(t, storeutil.MatchEntityQuery("page", "PAGE-1", def, q, nil))
	})

	t.Run("type and id filters still apply", func(t *testing.T) {
		q := store.EntityQuery{Type: "page"}
		assert.False(t, storeutil.MatchEntityQuery("ticket", "TKT-1", def, q, nil))

		idSet := map[string]bool{"PAGE-1": true}
		assert.True(t, storeutil.MatchEntityQuery("page", "PAGE-1", def, q, idSet))
		assert.False(t, storeutil.MatchEntityQuery("page", "PAGE-9", def, q, idSet))
	})
}

// A mixed-type family is unreachable through the write API (every
// backend rejects a state whose type differs from its family's), but
// fsstore builds its index from directory structure without reading
// files, so a hand-edited tree can present one. The answer must not
// depend on the order candidates happen to arrive in.
func TestWorldPrimes_MixedTypeFamilyIsOrderIndependent(t *testing.T) {
	w := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: []entity.Pointer{"published"}, Fallback: store.FallbackExclude},
	})
	def := storeutil.WorldCandidate{ID: "A", Type: "other", Pointer: ""}
	pub := storeutil.WorldCandidate{ID: "A", Type: "page", Pointer: "published"}

	forward := storeutil.WorldPrimes(w, []storeutil.WorldCandidate{def, pub})
	reverse := storeutil.WorldPrimes(w, []storeutil.WorldCandidate{pub, def})

	assert.Equal(t, forward, reverse,
		"candidate order must not change the verdict for a mixed-type family")
}
