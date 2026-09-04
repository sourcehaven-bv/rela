package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunWorldTests is the world-resolution conformance suite (TKT-WAV8XP):
// the three resolution rules, chain order, the at-most-one-prime
// invariant, exclude-vs-default fallback, the zero-value default-world
// fast path, and the AllStates+World refusal.
//
// These cases define the contract BEFORE the second backend implements
// it. fs/mem resolve in shared Go (storeutil); pgstore resolves in SQL
// in PR-C — this suite is what stops the two from drifting, exactly as
// RunStateTests does for content states.
//
// Gated by Capabilities.Worlds for one commit window; see that field.
func RunWorldTests(t *testing.T, f Factory) {
	ptr := func(t *testing.T, v string) entity.Face {
		t.Helper()
		p, err := entity.ParseFace(v)
		require.NoError(t, err)
		return p
	}
	newState := func(t *testing.T, id, typ, p, title string) *entity.Entity {
		t.Helper()
		e := entity.New(id, typ)
		if p != "" {
			e.Face = ptr(t, p)
		}
		e.SetString("title", title)
		return e
	}
	mustCreate := func(t *testing.T, s store.Store, e *entity.Entity) {
		t.Helper()
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
	// scope builds a one-type world so each case states its own rule.
	scope := func(t *testing.T, typ string, fb store.Fallback, chain ...string) store.WorldScope {
		t.Helper()
		coords := make([]entity.Face, 0, len(chain))
		for _, c := range chain {
			coords = append(coords, ptr(t, c))
		}
		return store.NewWorldScope(map[string]store.TypeResolution{
			typ: {Chain: coords, Fallback: fb},
		})
	}
	// coordScope is scope's sibling for chains containing the ZERO
	// coordinate, which `scope` structurally cannot build: it maps every
	// element through entity.ParseFace, and that codec REJECTS the empty
	// string (it is not a parseable name — it is the absence of one).
	//
	// The shape is reachable in production: a face declared `default:
	// true` is stored under the zero face, so a world selecting it by name
	// compiles to a chain carrying "" (internal/worlds, BUG-DFLTCHAIN). Before
	// that fix the compiler emitted the literal name instead, which no row
	// could match — so this suite never saw a zero-coordinate chain and could
	// not have caught the divergence if one existed.
	//
	// Taking []entity.Face directly rather than widening `scope` keeps the
	// common case honest: a test naming coordinates as STRINGS still goes
	// through the codec, so a typo in an ordinary chain is still caught.
	coordScope := func(typ string, fb store.Fallback, chain ...entity.Face) store.WorldScope {
		return store.NewWorldScope(map[string]store.TypeResolution{
			typ: {Chain: chain, Fallback: fb},
		})
	}
	// titles collects the resolved faces, keyed by bare id, and asserts
	// the at-most-one-prime invariant on the way through.
	titles := func(t *testing.T, s store.Store, q store.EntityQuery) map[string]string {
		t.Helper()
		out := map[string]string{}
		for e, err := range s.ListEntities(ctx(), q) {
			require.NoError(t, err)
			if prev, dup := out[e.ID]; dup {
				t.Fatalf("at-most-one-prime violated: %s resolved to both %q and %q",
					e.ID, prev, e.GetString("title"))
			}
			out[e.ID] = e.GetString("title")
		}
		return out
	}

	// Rule 2: the first coordinate that EXISTS wins, and ordering is the
	// whole semantic content of a chain — the same world must pick
	// different states for different entities.
	t.Run("Rule2_FirstExistingCoordinateWins", func(t *testing.T) {
		s := f(t)
		// PAGE-1 holds both; PAGE-2 holds only the later coordinate.
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "1 default"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "review", "1 review"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "1 published"))
		mustCreate(t, s, newState(t, "PAGE-2", "page", "", "2 default"))
		mustCreate(t, s, newState(t, "PAGE-2", "page", "published", "2 published"))

		got := titles(t, s, store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackExclude, "review", "published"),
		})
		assert.Equal(t, map[string]string{
			"PAGE-1": "1 review",    // review outranks published
			"PAGE-2": "2 published", // no review row, so published wins
		}, got, "a chain is a ranked preference, not a set")
	})

	// The inverse chain over the same data must give the other answer,
	// or the implementation is matching a SET and the order is decorative.
	t.Run("Rule2_ChainOrderIsLoadBearing", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "review", "review face"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "published face"))

		forward := titles(t, s, store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackExclude, "review", "published"),
		})
		reverse := titles(t, s, store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackExclude, "published", "review"),
		})
		assert.Equal(t, "review face", forward["PAGE-1"])
		assert.Equal(t, "published face", reverse["PAGE-1"],
			"reversing the chain must reverse the winner")
	})

	// Rule 3 under `otherwise: exclude` — absence IS the publication bit.
	// An entity with no coordinate in the chain is not in the world at
	// all; it must not fall back to its default face.
	t.Run("Rule3_ExcludeOmitsTheEntity", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "1 default"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "1 published"))
		mustCreate(t, s, newState(t, "PAGE-2", "page", "", "2 default")) // no published row

		got := titles(t, s, store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackExclude, "published"),
		})
		assert.Equal(t, map[string]string{"PAGE-1": "1 published"}, got)
		assert.NotContains(t, got, "PAGE-2",
			"an unpublished entity must be ABSENT, not served as its draft")
	})

	// Rule 3 under `otherwise: default` — the same data, the opposite
	// verdict. This pair is why the fallback is a verdict and not a
	// chain suffix.
	t.Run("Rule3_DefaultFallsBackToTheDefaultState", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "1 default"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "1 published"))
		mustCreate(t, s, newState(t, "PAGE-2", "page", "", "2 default"))

		got := titles(t, s, store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackDefaultState, "published"),
		})
		assert.Equal(t, map[string]string{
			"PAGE-1": "1 published",
			"PAGE-2": "2 default",
		}, got)
	})

	// Rule 1: a type ABSENT from the scope contributes its default state
	// in every world. Absence and the zero TypeResolution mean opposite
	// things, and this is the case that tells them apart — a backend
	// reading a map-miss as the zero value would drop `ticket` entirely.
	t.Run("Rule1_UnscopedTypeContributesItsDefaultState", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "page default"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "page published"))
		mustCreate(t, s, newState(t, "TKT-1", "ticket", "", "ticket default"))

		// A world naming only `page`; `ticket` is absent from the map.
		got := titles(t, s, store.EntityQuery{
			World: scope(t, "page", store.FallbackExclude, "published"),
		})
		assert.Equal(t, "page published", got["PAGE-1"])
		assert.Equal(t, "ticket default", got["TKT-1"],
			"a type the world does not name keeps its default state (rule 1)")
	})

	// The at-most-one-prime invariant, stated on its own. `face IN
	// (...)` would return two rows for PAGE-1 and is the obvious wrong
	// implementation.
	t.Run("AtMostOnePrimePerEntity", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "review", "review"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "published"))

		q := store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackDefaultState, "review", "published"),
		}
		titles(t, s, q) // fatals on a duplicate id

		n, err := s.CountEntities(ctx(), q)
		require.NoError(t, err)
		assert.Equal(t, 1, n, "an entity holding three faces contributes exactly one row")
	})

	// The zero WorldScope is the DEFAULT WORLD, and must be
	// byte-identical to the pre-worlds query — this is what keeps every
	// existing construction site and every faceless project free.
	t.Run("ZeroWorldIsTodaysBehavior", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "default face"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "published face"))

		bare := titles(t, s, store.EntityQuery{Type: "page"})
		zero := titles(t, s, store.EntityQuery{Type: "page", World: store.DefaultWorld()})
		assert.Equal(t, map[string]string{"PAGE-1": "default face"}, bare)
		assert.Equal(t, bare, zero, "the zero WorldScope must not change any result")
	})

	// A project that never declares a face must be untouched by the
	// feature existing.
	t.Run("FacelessProjectIsUnaffected", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "TKT-1", "ticket", "", "one"))
		mustCreate(t, s, newState(t, "TKT-2", "ticket", "", "two"))

		want := map[string]string{"TKT-1": "one", "TKT-2": "two"}
		assert.Equal(t, want, titles(t, s, store.EntityQuery{Type: "ticket"}))
		assert.Equal(t, want, titles(t, s, store.EntityQuery{
			Type:  "ticket",
			World: scope(t, "page", store.FallbackExclude, "published"),
		}), "a world scoping another type must not touch this one")
	})

	// A non-default world must actually return a NON-DEFAULT row. The
	// candidate filter is WIDENED under a world (every state of a family
	// becomes a candidate, and resolution picks among them afterwards);
	// if someone later "optimizes" that back to a default-only filter,
	// every world silently resolves to default faces and this is the
	// only case that says so. The failure mode is silence in the
	// direction of serving the wrong face: a published world that quietly
	// shows drafts reads exactly like a correctly-working one.
	t.Run("NonDefaultWorldReturnsANonDefaultState", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "the draft nobody should see"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "the published face"))

		q := store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackExclude, "published"),
		}
		var got []*entity.Entity
		for e, err := range s.ListEntities(ctx(), q) {
			require.NoError(t, err)
			got = append(got, e)
		}
		require.Len(t, got, 1)
		assert.Equal(t, "the published face", got[0].GetString("title"))
		assert.False(t, got[0].Face.IsDefault(),
			"the resolved row must be the STATE row, not the default face")
		assert.Equal(t, ptr(t, "published"), got[0].Face)
	})

	// Families must stay CONTIGUOUS in the backend's ordering, or a page
	// boundary can split one and the resolver decides a prime having seen
	// only part of it. Because the fallback verdict is a decision about
	// ABSENCE, a partial view yields a WRONG prime, not a slow one.
	//
	// The ids matter: '@' is 0x40 and digits are 0x30-0x39, so under
	// plain string ordering of the joined "id@face" key, PAGE-10 sorts
	// BETWEEN PAGE-1 and PAGE-1@draft and splits PAGE-1's family. This
	// case is why fs/mem sort by the (bare id, face) tuple; a refactor
	// back to concatenated keys dies here.
	t.Run("FamiliesStayContiguousAcrossPageBoundaries", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"PAGE-1", "PAGE-10", "PAGE-2"} {
			mustCreate(t, s, newState(t, id, "page", "", id+" default"))
			mustCreate(t, s, newState(t, id, "page", "published", id+" published"))
		}

		// FallbackDefaultState is the discriminating verdict. Under plain
		// string ordering PAGE-1's family splits into two NON-ADJACENT
		// runs (`PAGE-1`, then `PAGE-10`'s whole family, then
		// `PAGE-1@published`), so a family-buffering resolver decides
		// PAGE-1 TWICE: the first run sees only the default row, fires
		// the fallback, and emits the draft face; the second run sees
		// only the published row and emits that. The entity appears
		// twice, and one of the two rows is the face the world was
		// supposed to replace. With `exclude` both runs happen to agree,
		// which is why this case must use `default`.
		world := scope(t, "page", store.FallbackDefaultState, "published")
		want := map[string]string{
			"PAGE-1":  "PAGE-1 published",
			"PAGE-10": "PAGE-10 published",
			"PAGE-2":  "PAGE-2 published",
		}

		// Unpaged is the oracle.
		assert.Equal(t, want, titles(t, s, store.EntityQuery{Type: "page", World: world}))

		// Paging one prime at a time must produce the same set: every
		// page boundary falls between two families.
		for _, limit := range []int{1, 2, 3} {
			got := map[string]string{}
			cursor := ""
			for range 10 { // bounded: 3 primes, so this always terminates
				page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{
					Type: "page", World: world, Limit: limit, Cursor: cursor,
				})
				require.NoError(t, err)
				for _, e := range page.Items {
					if prev, dup := got[e.ID]; dup {
						t.Fatalf("limit %d: %s yielded twice (%q then %q)",
							limit, e.ID, prev, e.GetString("title"))
					}
					got[e.ID] = e.GetString("title")
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
			assert.Equal(t, want, got, "paging with limit %d must not drop or split a family", limit)
		}
	})

	// The header path must resolve the world too. ListEntityHeaders is an
	// OPTIONAL capability (store.HeaderReader): memstore and pgstore
	// implement it natively, fsstore does not and is served by the generic
	// store.ListEntityHeaders fallback over ListEntities. Both arms are
	// exercised here by going through the package-level helper, which is
	// what every caller uses.
	//
	// Worth stating why this case exists: the header suite in header.go
	// passes no World, and RunWorldTests otherwise covers the other six
	// query entry points — so without this, a natively-implemented header
	// path could serve the default world while the list beside it serves
	// the requested one, and no conformance case would notice. That gap
	// matters most for pgstore, whose PR-B refusal is what currently
	// stands in for this coverage (TKT-WAV8XP PR-C deletes the refusal).
	t.Run("HeaderPathHonorsTheWorld", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "HDR-1", "page", "", "HDR-1 draft"))
		mustCreate(t, s, newState(t, "HDR-1", "page", "published", "HDR-1 published"))
		// HDR-2 has no published state, so `exclude` must drop it.
		mustCreate(t, s, newState(t, "HDR-2", "page", "", "HDR-2 draft only"))

		q := store.EntityQuery{
			Type:  "page",
			World: scope(t, "page", store.FallbackExclude, "published"),
		}
		got := map[string]string{}
		for h, err := range store.ListEntityHeaders(ctx(), s, q) {
			require.NoError(t, err)
			if prev, dup := got[h.ID]; dup {
				t.Fatalf("at-most-one-prime violated: %s resolved to both %q and %q",
					h.ID, prev, h.Properties["title"])
			}
			title, _ := h.Properties["title"].(string)
			got[h.ID] = title
		}
		assert.Equal(t, map[string]string{"HDR-1": "HDR-1 published"}, got,
			"headers must be world-scoped: the published face only, and no excluded entity")
	})

	// A world must reach the GRAPH-query path as well as the list path.
	// The ACL read path swaps an EntityQuery for a GraphQuery the moment
	// a policy query exists (internal/visibility/pushdown.go), so a world
	// honored on only one of the two fails OPEN for exactly the gated
	// principals: `otherwise: exclude` stops hiding, and a published
	// world starts serving drafts. Pinned here rather than per-backend
	// because all three delegate the seed to the same helper.
	t.Run("GraphQueryHonorsTheWorld", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "GQ-1", "page", "", "GQ-1 default"))
		mustCreate(t, s, newState(t, "GQ-1", "page", "published", "GQ-1 published"))
		// GQ-2 holds no published state, so `exclude` must drop it.
		mustCreate(t, s, newState(t, "GQ-2", "page", "", "GQ-2 default only"))

		world := scope(t, "page", store.FallbackExclude, "published")
		q := store.GraphQuery{EntityType: "page", World: world}

		got := map[string]string{}
		for e, err := range s.GraphQuery(ctx(), q) {
			require.NoError(t, err)
			if prev, dup := got[e.ID]; dup {
				t.Fatalf("at-most-one-prime violated: %s resolved to both %q and %q",
					e.ID, prev, e.GetString("title"))
			}
			got[e.ID] = e.GetString("title")
		}
		assert.Equal(t, map[string]string{"GQ-1": "GQ-1 published"}, got,
			"GraphQuery must return primes, not default states")

		// The same scope must reach the count and membership paths, or
		// a gated list and its total disagree.
		matched, total, err := s.GraphCount(ctx(), q)
		require.NoError(t, err)
		assert.Equal(t, 1, matched, "GraphCount matched")
		assert.Equal(t, 1, total, "GraphCount total counts world-scoped candidates")

		ids, err := s.MatchingIDs(ctx(), q, []string{"GQ-1", "GQ-2"})
		require.NoError(t, err)
		assert.Equal(t, map[string]bool{"GQ-1": true, "GQ-2": false}, ids,
			"an entity the world excludes must not match")
	})

	// AllStates and World are mutually exclusive (decision Q3): raw
	// storage truth versus resolution. The refusal is shared so every
	// backend inherits it; silently honoring one would be a precedence
	// rule nobody remembers.
	// A chain carrying the ZERO coordinate must be matched at its own RANK,
	// like any other coordinate — not diverted into the rule-1/rule-3 path by
	// a default-ness special case.
	//
	// This is the shape internal/worlds newly emits for a world that selects a
	// `bare_face` face by name (BUG-DFLTCHAIN). It is a CROSS-BACKEND
	// contract because the two implementations reach the answer differently:
	// storeutil.WorldPrimes has an explicit `Face.IsDefault()` branch that
	// must fall THROUGH to the rank loop rather than short-circuit, while
	// pgstore builds a CASE whose arms rank the coordinates — and under
	// `otherwise: default` that CASE ends up with two arms matching the same
	// row, so it relies on first-match-wins agreeing with the Go ranking.
	//
	// Neither property is obvious, both are currently correct, and until this
	// case existed nothing held them together: the suite could not construct
	// the chain at all, so a future edit to either side could have diverged
	// while every test stayed green.
	//
	// Both fallbacks are exercised, and the `exclude` arm is what gives the
	// case teeth: under `otherwise: default` a zero-coordinate row would be
	// served either way — by the chain if the rank matched, by the fallback if
	// it did not — so the two outcomes are indistinguishable. Under `exclude`
	// there is no fallback to hide behind, so serving PAGE-2 at all proves the
	// rank actually matched. Do not "simplify" this to the default arm alone.
	//
	// Mutation-checked (Ruling 10): adding a `continue` to WorldPrimes'
	// IsDefault branch — diverting the default row past the rank loop, which
	// is the exact divergence described above — fails THIS CASE AND NOTHING
	// ELSE across the whole conformance suite. That is the measure of the gap
	// it closes.
	t.Run("Rule2_ZeroCoordinateIsRankedLikeAnyOther", func(t *testing.T) {
		const def = entity.Face("")
		for _, fb := range []struct {
			name string
			fb   store.Fallback
		}{
			{"exclude", store.FallbackExclude},
			{"default", store.FallbackDefaultState},
		} {
			t.Run(fb.name, func(t *testing.T) {
				s := f(t)
				// PAGE-1 holds both faces; PAGE-2 only its default.
				mustCreate(t, s, newState(t, "PAGE-1", "page", "", "1 default"))
				mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "1 published"))
				mustCreate(t, s, newState(t, "PAGE-2", "page", "", "2 default"))

				// The zero coordinate LAST: published outranks it for PAGE-1,
				// and PAGE-2 is still served — via the CHAIN, not the fallback,
				// which is why the exclude arm must return it too.
				last := titles(t, s, store.EntityQuery{
					Type:  "page",
					World: coordScope("page", fb.fb, "published", def),
				})
				assert.Equal(t, "1 published", last["PAGE-1"],
					"a higher-ranked coordinate still wins over the zero one")
				assert.Equal(t, "2 default", last["PAGE-2"],
					"the zero coordinate is IN the chain, so PAGE-2 resolves by "+
						"rule 2 — under `exclude` there is no fallback that could "+
						"have produced this, so serving it proves the rank matched")

				// The zero coordinate FIRST: it now outranks published.
				first := titles(t, s, store.EntityQuery{
					Type:  "page",
					World: coordScope("page", fb.fb, def, "published"),
				})
				assert.Equal(t, "1 default", first["PAGE-1"],
					"chain ORDER governs the zero coordinate exactly as it does "+
						"any other — reversing the chain reverses the winner")
				assert.Equal(t, "2 default", first["PAGE-2"])
			})
		}
	})

	// FaceIn is the ACL's face allowlist. The graph-query path (GraphQuery /
	// MatchingIDs) is what the read gate swaps in as soon as a policy query
	// exists, so a backend that honors FaceIn on ListEntities but drops it
	// here FAILS OPEN for exactly the gated principals — the naive graph
	// query did, on fs/mem/sqlite.
	t.Run("FaceInNarrowsGraphQueryAndMatchingIDs", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "P-1", "page", "", "draft"))
		mustCreate(t, s, newState(t, "P-1", "page", "review", "review"))
		mustCreate(t, s, newState(t, "P-2", "page", "", "draft two"))
		mustCreate(t, s, newState(t, "P-2", "page", "published", "published"))

		// Under the default world the only candidate is the bare row, so the
		// allowlist has to be exercised together with a world that selects
		// the named face — which is also the production shape: a face grant
		// compiles to FaceIn beside the request's World.
		world := scope(t, "page", store.FallbackDefaultState, "published")
		denied := store.GraphQuery{
			EntityType: "page", World: world, FaceIn: []entity.Face{ptr(t, "published")},
		}
		var got []string
		for e, err := range s.GraphQuery(ctx(), denied) {
			require.NoError(t, err)
			got = append(got, e.ID+"@"+e.Face.String())
		}
		assert.Equal(t, []string{"P-2@published"}, got,
			"only rows in the allowlisted face may match; P-1 has no published face")
		m, err := s.MatchingIDs(ctx(), denied, []string{"P-1", "P-2"})
		require.NoError(t, err)
		assert.Equal(t, map[string]bool{"P-1": false, "P-2": true}, m,
			"MatchingIDs is the per-id read verdict; P-1 must be denied")

		// Positive control on the ZERO face under the same world: both
		// entities have a bare row, and FaceIn is applied before the rank, so
		// P-2's excluded published face falls through to it. Without this the
		// case would pass against a backend that returns nothing for every
		// FaceIn.
		bare := store.GraphQuery{EntityType: "page", World: world, FaceIn: []entity.Face{""}}
		m, err = s.MatchingIDs(ctx(), bare, []string{"P-1", "P-2"})
		require.NoError(t, err)
		assert.Equal(t, map[string]bool{"P-1": true, "P-2": true}, m)
	})

	// A page cursor names the last PRIME. Resuming at the next KEY rather
	// than the next FAMILY re-buffers the cursor entity's remaining rows as
	// a partial family, which resolves to whatever it still holds — so with
	// `select: [draft, published]` PAGE-1 came back on page 2 as its
	// published face. The sibling case above cannot catch this: there the
	// chain face sorts LAST within the family.
	t.Run("PagingResumesAtTheNextFamily", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "PAGE-1 bare"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "draft", "PAGE-1 draft"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "published", "PAGE-1 published"))
		mustCreate(t, s, newState(t, "PAGE-2", "page", "", "PAGE-2 bare"))
		mustCreate(t, s, newState(t, "PAGE-2", "page", "draft", "PAGE-2 draft"))
		world := scope(t, "page", store.FallbackDefaultState, "draft", "published")

		want := map[string]string{"PAGE-1": "PAGE-1 draft", "PAGE-2": "PAGE-2 draft"}
		assert.Equal(t, want, titles(t, s, store.EntityQuery{Type: "page", World: world}))

		got := map[string]string{}
		cursor := ""
		for range 10 {
			page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{
				Type: "page", World: world, Limit: 1, Cursor: cursor,
			})
			require.NoError(t, err)
			for _, e := range page.Items {
				if prev, dup := got[e.ID]; dup {
					t.Fatalf("%s yielded twice (%q then %q): the resume landed mid-family",
						e.ID, prev, e.GetString("title"))
				}
				got[e.ID] = e.GetString("title")
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		assert.Equal(t, want, got)
	})

	// Any is the per-relation face grant (store.GraphQuery.Any): each branch
	// pairs a relation predicate with the faces THAT relation's role grants,
	// applied to the candidates before the rank like FaceIn. The ACL compiles
	// conferred roles to this shape so an owner's draft grant cannot be
	// laundered through a reviewer's edge. Pinned here on every backend
	// because the SQL and the naive evaluator implement it separately.
	t.Run("AnyBranchesGrantFacesPerRelation", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "P-1", "page", "", "bare"))
		mustCreate(t, s, newState(t, "P-1", "page", "published", "published"))
		mustCreate(t, s, newState(t, "U-owner", "user", "", "owner"))
		mustCreate(t, s, newState(t, "U-reviewer", "user", "", "reviewer"))
		_, err := s.CreateRelation(ctx(), "U-owner", "owns", "P-1", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "U-reviewer", "reviews", "P-1", nil)
		require.NoError(t, err)

		branches := func(who string) []store.GraphBranch {
			return []store.GraphBranch{
				{HasInbound: &store.RelationPredicate{Endpoints: []string{who}, OfTypes: []string{"owns"}},
					FaceIn: []entity.Face{""}},
				{HasInbound: &store.RelationPredicate{Endpoints: []string{who}, OfTypes: []string{"reviews"}},
					FaceIn: []entity.Face{ptr(t, "published")}},
			}
		}
		published := scope(t, "page", store.FallbackExclude, "published")
		for _, tc := range []struct {
			name  string
			who   string
			world store.WorldScope
			want  bool
		}{
			// The owner's role grants the bare face only, so under the default
			// world (bare rows) the owner reads P-1 ...
			{"owner reads the bare face in the default world", "U-owner", store.DefaultWorld(), true},
			// ... and the reviewer, whose role grants only published, does NOT
			// — even though a union of both roles' faces would include it.
			{"reviewer cannot read the bare face", "U-reviewer", store.DefaultWorld(), false},
			// Under a world selecting published, the reviewer reads the
			// published prime ...
			{"reviewer reads the published prime", "U-reviewer", published, true},
			// ... and the owner does not: the owns branch keeps only the bare
			// candidate, which the chain does not select and `exclude` drops.
			{"owner has no candidate the published world selects", "U-owner", published, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				q := store.GraphQuery{EntityType: "page", World: tc.world, Any: branches(tc.who)}
				m, err := s.MatchingIDs(ctx(), q, []string{"P-1"})
				require.NoError(t, err)
				assert.Equal(t, tc.want, m["P-1"], "MatchingIDs")
				var got []string
				for e, err := range s.GraphQuery(ctx(), q) {
					require.NoError(t, err)
					got = append(got, e.ID+"@"+e.Face.String())
				}
				if tc.want {
					assert.Len(t, got, 1, "GraphQuery must return exactly the prime")
				} else {
					assert.Empty(t, got)
				}
			})
		}
	})

	t.Run("AllStatesWithWorldIsRejected", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "default"))

		q := store.EntityQuery{
			Type:      "page",
			AllStates: true,
			World:     scope(t, "page", store.FallbackExclude, "published"),
		}
		var got error
		for _, err := range s.ListEntities(ctx(), q) {
			if err != nil {
				got = err
				break
			}
		}
		assert.ErrorIs(t, got, store.ErrInvalidQuery,
			"a contradictory query must be refused, not silently resolved")

		_, err := s.CountEntities(ctx(), q)
		assert.ErrorIs(t, err, store.ErrInvalidQuery)
	})
}
