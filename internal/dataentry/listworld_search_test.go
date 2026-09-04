package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Free-text search under a world (TKT-9KZGJO step 5).
//
// These run against the REAL searcher newTestAppV1 wires (a bleve index
// observing the memstore, keyed per face), not a stub. A stub could be made to
// return whatever the assertion wanted; the claim here is about which face the
// index actually scored and which entity the resolution actually dropped, and
// only a real index can make it.
//
// The three properties, and why each earns a test:
//
//   - A world's search returns the world's PRIME face, so a hit's text is the
//     text the reader will be shown.
//   - An entity with NO face in the world is absent from its search, because
//     under `otherwise: exclude` absence IS the publication bit.
//   - A DENIED world leaks nothing through `?q=`, which is the one path that
//     could route around the read gate the middleware applies.

// searchWorlds is a world lookup for these tests: `published` selects the
// published face of a ticket and EXCLUDES a ticket that has none.
//
// Deliberately not stubWorlds{}: that fixture's `published` shape is right, but
// these tests also need `?world=` to arrive through the real middleware with a
// grant check, so they use the router rather than a hand-built handle.
type searchWorlds struct{}

func (searchWorlds) Lookup(name string) (store.WorldScope, bool) {
	if name != "published" {
		return store.WorldScope{}, false
	}
	return store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {
			Chain:    []entity.Face{entity.Face("published")},
			Fallback: store.FallbackExclude,
		},
	}), true
}

// seedFacedTickets seeds the corpus every test below shares:
//
//	TKT-PUB   draft "sardine onboarding"   published "walrus onboarding"
//	TKT-DRAFT draft "sardine offboarding"   (no published face)
//
// The vocabulary is nonsense on purpose. Real words would be plausible
// substrings of each other and of the ids, and a bleve hit that came from the
// wrong place would still look like a pass.
func seedFacedTickets(t *testing.T, app *App) {
	t.Helper()
	seedEntity(app, &entity.Entity{
		ID: "TKT-PUB", Type: "ticket",
		Properties: map[string]any{"title": "sardine onboarding"},
	})
	if err := app.store.CreateEntity(context.Background(), &entity.Entity{
		ID: "TKT-PUB", Type: "ticket", Face: entity.Face("published"),
		Properties: map[string]any{"title": "walrus onboarding"},
	}); err != nil {
		t.Fatalf("seed published face: %v", err)
	}
	seedEntity(app, &entity.Entity{
		ID: "TKT-DRAFT", Type: "ticket",
		Properties: map[string]any{"title": "sardine offboarding"},
	})
	app.SetWorlds(searchWorlds{})
}

// searchIDs runs a list request through the real router and returns the ids.
func searchIDs(t *testing.T, app *App, path string) []string {
	t.Helper()
	rows := listRows(t, app, path)
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		id, _ := r["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func assertIDs(t *testing.T, got, want []string, why string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got ids %v, want %v — %s", got, want, why)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got ids %v, want %v — %s", got, want, why)
		}
	}
}

// TestListSearch_MatchesTheWorldsPrimeFace is the positive claim: under
// `published`, the text that decides a hit is the PUBLISHED face's text.
//
// Both directions are asserted, and the second is the one that matters. A term
// present only in the published face must HIT (proving the world's face was
// searched at all), and a term present only in the draft must MISS (proving the
// draft was not also searched). Only the first would pass on an implementation
// that searched every face and unioned the results — which is exactly the
// draft-leak this closes.
func TestListSearch_MatchesTheWorldsPrimeFace(t *testing.T) {
	app := newTestAppV1(t)
	seedFacedTickets(t, app)

	t.Run("published-only term hits", func(t *testing.T) {
		got := searchIDs(t, app, "/api/v1/tickets?world=published&q=walrus")
		assertIDs(t, got, []string{"TKT-PUB"},
			"`walrus` is in TKT-PUB's PUBLISHED face, which is what the "+
				"published world resolves to — so the world's own face was searched")
	})

	t.Run("draft-only term misses", func(t *testing.T) {
		got := searchIDs(t, app, "/api/v1/tickets?world=published&q=sardine")
		assertIDs(t, got, nil,
			"`sardine` appears ONLY in draft faces. A hit here means the index "+
				"matched a face the world did not resolve to, so a reader could "+
				"search up draft wording while being shown published bytes")
	})

	t.Run("the default world still sees the draft", func(t *testing.T) {
		// The control. Without it the test above passes on a searcher that is
		// simply broken, and "no hits" would look like correct scoping.
		got := searchIDs(t, app, "/api/v1/tickets?q=sardine")
		assertIDs(t, got, []string{"TKT-DRAFT", "TKT-PUB"},
			"the default world addresses every entity at its default (draft) "+
				"face, so `sardine` matches both — this proves the miss above "+
				"is scoping, not a dead searcher")
	})
}

// TestListSearch_EntityWithNoFaceInTheWorldIsAbsent pins the publication bit
// on the search path.
//
// TKT-DRAFT has no published face, so under `otherwise: exclude` it resolves
// to nothing and contributes nothing. `offboarding` is a term unique to it, so
// a hit could only mean the excluded entity was searched anyway — and a search
// box that surfaces the TITLE of an entity a reader may not see discloses it
// just as completely as listing it would.
func TestListSearch_EntityWithNoFaceInTheWorldIsAbsent(t *testing.T) {
	app := newTestAppV1(t)
	seedFacedTickets(t, app)

	got := searchIDs(t, app, "/api/v1/tickets?world=published&q=offboarding")
	assertIDs(t, got, nil,
		"TKT-DRAFT has no published face, so it is ABSENT from that world — "+
			"including from its search, which is otherwise a title oracle over "+
			"exactly the entities `otherwise: exclude` exists to hide")

	// Same term, default world: present. Proves the absence is the world's
	// doing rather than an indexing gap.
	got = searchIDs(t, app, "/api/v1/tickets?q=offboarding")
	assertIDs(t, got, []string{"TKT-DRAFT"},
		"the entity IS indexed and findable in the default world")
}

// TestListSearch_DeniedWorldFindsNothing pins that `?q=` cannot become the way
// around the per-world read grant.
//
// # Why this asserts on the seam and not only through the handler
//
// The end-to-end path is already safe twice over, and the outer guard is the
// STRONGER one: scopedSortedEntities returns an empty slice on a denied handle
// before any search runs, so the handler could not leak even if the search
// itself were unscoped. Asserting only through the handler therefore proves
// nothing about the search — the assertion passes with the search guard
// deleted, which is a green test covering no code.
//
// So both are asserted separately. The handler claim is the property a reader
// of the API cares about; the seam claim is that freeTextIDsForType is safe ON
// ITS OWN, which is what stops a future caller (one without the early return)
// from turning a denied world into a full default-world search. That is not
// hypothetical framing: a denied handle carries the ZERO scope, because
// resolveWorld never built one for it — and a zero scope IS the default world.
// Stamping it without checking `denied` searches everything.
func TestListSearch_DeniedWorldFindsNothing(t *testing.T) {
	app := newTestAppV1(t)
	seedFacedTickets(t, app)

	// The seam, called directly with a denied handle. `sardine` matches in the
	// default world, so an unscoped run is visible as a non-empty id set.
	denied := withWorld(withReadGate(aliceCtx(), nopReadGate{}),
		worldHandle{name: "published", denied: true})
	res, err := app.queries.freeTextIDsForType(denied, "sardine", "ticket")
	if err != nil {
		t.Fatalf("freeTextIDsForType: %v", err)
	}
	if !res.HasFilter {
		t.Fatal("a denied world must still report HasFilter — the caller " +
			"intersects on it, and HasFilter=false means `skip the intersection`, " +
			"which would return the UNFILTERED list rather than nothing")
	}
	if len(res.IDs) != 0 {
		t.Fatalf("a denied world must find nothing; got %v. The denied handle "+
			"carries the zero scope, which is the DEFAULT world — so stamping it "+
			"without the blocksAllReads check runs a full default-world search "+
			"for a principal refused that world entirely", res.IDs)
	}

	// And end to end: the ordinary handler still renders the ordinary empty
	// result, so a denial stays indistinguishable from an empty world.
	got := searchIDsUnderWorld(t, app, "sardine", worldHandle{name: "published", denied: true})
	if len(got) != 0 {
		t.Fatalf("a denied world must yield NOTHING through `?q=`; got %v", got)
	}
}

// searchIDsUnderWorld drives the list handler with an explicit world handle,
// which is the only way to reach the `denied` state: resolveWorld returns it as
// an error, and attachWorld constructs the handle itself.
func searchIDsUnderWorld(t *testing.T, app *App, q string, h worldHandle) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?q="+q, http.NoBody)
	ctx := withWorld(withReadGate(aliceCtx(), nopReadGate{}), h)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req.WithContext(ctx), "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("list under world: got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ids := make([]string, 0, len(resp.Data))
	for _, r := range resp.Data {
		id, _ := r["id"].(string)
		ids = append(ids, id)
	}
	return ids
}
