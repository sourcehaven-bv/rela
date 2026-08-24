package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// publishedViewWorld is the ISMS-shaped world for view tests: select the
// `published` face, exclude anything that has none.
func publishedViewWorld(types ...string) viewWorld {
	res := make(map[string]store.TypeResolution, len(types))
	for _, typ := range types {
		res[typ] = store.TypeResolution{
			Chain:    []entity.Pointer{entity.Pointer("published")},
			Fallback: store.FallbackExclude,
		}
	}
	return viewWorld{name: "published", scope: store.NewWorldScope(res)}
}

// seedViewGraph builds the fixture the world view tests share:
//
//	TKT-1 (draft + published) --implements--> FEAT-PUB   (draft + published)
//	                          --implements--> FEAT-DRAFT (draft only)
//
// Under `published`, FEAT-DRAFT has no face and must vanish from collections.
func seedViewGraph(t *testing.T, app *App) {
	t.Helper()
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "draft entry", "status": "open"},
	})
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Pointer: entity.Pointer("published"),
		Properties: map[string]any{"title": "published entry", "status": "closed"},
	}); err != nil {
		t.Fatalf("seed TKT-1@published: %v", err)
	}

	seedEntity(app, &entity.Entity{
		ID: "FEAT-PUB", Type: "feature",
		Properties: map[string]any{"title": "draft feature"},
	})
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "FEAT-PUB", Type: "feature", Pointer: entity.Pointer("published"),
		Properties: map[string]any{"title": "published feature"},
	}); err != nil {
		t.Fatalf("seed FEAT-PUB@published: %v", err)
	}
	seedEntity(app, &entity.Entity{
		ID: "FEAT-DRAFT", Type: "feature",
		Properties: map[string]any{"title": "unpublished feature"},
	})

	for _, to := range []string{"FEAT-PUB", "FEAT-DRAFT"} {
		if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", to, nil); err != nil {
			t.Fatalf("seed edge to %s: %v", to, err)
		}
	}
}

func implementsView() ViewConfig {
	return ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "implements", CollectAs: "features"},
		},
	}
}

// collectionIDs lists the ids collected under the fixture's one collection.
//
// The name is fixed rather than a parameter: every test here uses the same
// single-rule fixture, and a parameter that only ever receives one value is a
// generality nothing exercises.
func collectionIDs(result *viewResult) []string {
	var out []string
	for _, e := range result.Collections["features"] {
		out = append(out, e.ID)
	}
	return out
}

// TestExecuteView_EntryResolvesToTheWorldsFace pins that the view's ENTRY is
// the world's face, not the default state.
//
// This is the first thing item 4b had to fix: executeView read the entry via a
// raw store.GetEntity, which under any world yields the wrong object — the
// page would render draft content while claiming to be a published view.
//
// It asserts on the entry's PROPERTIES rather than its pointer, because the
// pointer being right while the content is stale is not a failure mode this
// code can produce, whereas a reader who only checked the pointer would not
// notice if the wrong row were returned.
//
// Mutation-checked: `viewWorld.isDefault() → true` fails this.
func TestExecuteView_EntryResolvesToTheWorldsFace(t *testing.T) {
	app := newTestAppV1(t)
	seedViewGraph(t, app)

	def, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-1", defaultViewWorld())
	if err != nil {
		t.Fatalf("default world: %v", err)
	}
	if got, _ := def.Entry.Properties["title"].(string); got != "draft entry" {
		t.Fatalf("precondition: the default world must serve the draft face, "+
			"or the world assertion below proves nothing; got %q", got)
	}

	pub, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-1",
		publishedViewWorld("ticket", "feature"))
	if err != nil {
		t.Fatalf("published world: %v", err)
	}
	if got, _ := pub.Entry.Properties["title"].(string); got != "published entry" {
		t.Errorf("the entry must be the WORLD's face; got %q, want %q",
			got, "published entry")
	}
	if pub.Entry.Pointer != entity.Pointer("published") {
		t.Errorf("entry pointer = %q, want published", pub.Entry.Pointer)
	}
}

// TestExecuteView_ExcludedCollectionEntityIsAbsent is the RULING 12 case for
// collections: a traversed neighbor with no face in this world must not
// appear, even though the EDGE to it exists in storage.
//
// The edge is genuinely there — traversal finds the id either way — so this
// fails if head resolution is skipped or done in the wrong world. That is
// exactly the shape item 4 fixed for the entity GET's relations map, now for
// view collections.
//
// Mutation-checked: dropping `World: w.scope` from loadViewEntities fails this.
func TestExecuteView_ExcludedCollectionEntityIsAbsent(t *testing.T) {
	app := newTestAppV1(t)
	seedViewGraph(t, app)

	def, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-1", defaultViewWorld())
	if err != nil {
		t.Fatalf("default world: %v", err)
	}
	if len(collectionIDs(def)) != 2 {
		t.Fatalf("precondition: the default world must collect BOTH features, "+
			"or the exclusion below is vacuous; got %v", collectionIDs(def))
	}

	pub, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-1",
		publishedViewWorld("ticket", "feature"))
	if err != nil {
		t.Fatalf("published world: %v", err)
	}
	got := map[string]bool{}
	for _, id := range collectionIDs(pub) {
		got[id] = true
	}
	if !got["FEAT-PUB"] {
		t.Errorf("a neighbor WITH a published face must be collected; got %v", got)
	}
	if got["FEAT-DRAFT"] {
		t.Errorf("a neighbor with NO published face must be ABSENT from the "+
			"collection — under otherwise:exclude it does not exist in this "+
			"world, so a link to it points at nothing (RULING 12); got %v", got)
	}
}

// TestExecuteView_CollectionEntitiesAreTheWorldsFaces pins that a collected
// entity is the world's FACE, not the default row that happens to share its id.
//
// Separate from the exclusion test because the two fail differently: a build
// that resolved heads in the DEFAULT world would pass the exclusion test's
// FEAT-PUB assertion (the id is present) while serving draft content under a
// published view — the mixed-face bug, which reads as correct.
func TestExecuteView_CollectionEntitiesAreTheWorldsFaces(t *testing.T) {
	app := newTestAppV1(t)
	seedViewGraph(t, app)

	pub, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-1",
		publishedViewWorld("ticket", "feature"))
	if err != nil {
		t.Fatalf("published world: %v", err)
	}
	for _, e := range pub.Collections["features"] {
		if e.ID != "FEAT-PUB" {
			continue
		}
		if got, _ := e.Properties["title"].(string); got != "published feature" {
			t.Errorf("a collected entity must be the WORLD's face, not the "+
				"default row with the same id; got title %q", got)
		}
		if e.Pointer != entity.Pointer("published") {
			t.Errorf("collected FEAT-PUB pointer = %q, want published", e.Pointer)
		}
		return
	}
	t.Fatal("FEAT-PUB missing from the collection — the precondition for this test")
}

// TestExecuteView_WhereFiltersTheResolvedFace is RULING 16, proved rather than
// asserted.
//
// The fixture's two faces differ in `status`: draft is `open`, published is
// `closed`. A `where: "status = closed"` therefore matches ONLY if the filter
// sees the published face. Filtering the draft values while rendering a
// published page would produce a collection that contradicts its own page.
//
// This is a TEST obligation rather than an implementation one: filterEntities
// reads e.Properties off whatever it is handed, so feeding it faces makes it
// filter faces. That is precisely why it needs a test — nothing in the filter
// code would look wrong if the faces stopped arriving.
//
// The entry itself is the subject here (`from: entry`), so the assertion is
// about the entry's own face reaching the filter.
func TestExecuteView_WhereFiltersTheResolvedFace(t *testing.T) {
	app := newTestAppV1(t)
	seedViewGraph(t, app)

	// A traversal filtered on a property whose value DIFFERS between the two
	// faces of the neighbor: FEAT-PUB is "draft feature" by default and
	// "published feature" on its published face.
	//
	// The filter value is UNQUOTED. internal/filter treats quotes as literal
	// characters rather than string delimiters, so `title = "x"` looks for a
	// title containing the quote marks and matches nothing — which is how the
	// first version of this test failed against correct code. Verified by
	// probing filterEntities directly with both forms.
	view := ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{
			From: "entry", Follow: "implements", CollectAs: "features",
			Where: `title = published feature`,
		}},
	}

	pub, err := app.views.executeView(
		context.Background(), view, "TKT-1", publishedViewWorld("ticket", "feature"))
	if err != nil {
		t.Fatalf("published world: %v", err)
	}
	if ids := collectionIDs(pub); len(ids) != 1 || ids[0] != "FEAT-PUB" {
		t.Errorf("`where:` must match against the RESOLVED FACE (RULING 16): "+
			"the published face's title is %q and the draft's is %q, so this "+
			"matches only if the filter sees faces rather than default rows; got %v",
			"published feature", "draft feature", ids)
	}

	// The same filter under the DEFAULT world matches nothing, because the
	// default rows carry the draft titles. This is the half that proves the
	// test is discriminating rather than passing on a coincidence.
	def, err := app.views.executeView(
		context.Background(), view, "TKT-1", defaultViewWorld())
	if err != nil {
		t.Fatalf("default world: %v", err)
	}
	if ids := collectionIDs(def); len(ids) != 0 {
		t.Errorf("under the default world the published title matches nothing; got %v", ids)
	}
}

// TestViewWorld_ProvenanceOnCollectionEntities pins the RULING 14 payload —
// the reason item 4b exists.
//
// Item 4 deferred per-neighbor provenance because the entity GET's
// `relations` map carries bare id STRINGS with nowhere to put it. A view
// collection carries whole ENTITIES, so the slot is natural, and this is where
// it lands.
//
// Asserts all three fields, and both worlds: present and correct under a
// world, ABSENT under the default one (where every entity is its default state
// by definition, so a block on every row would be noise).
func TestViewWorld_ProvenanceOnCollectionEntities(t *testing.T) {
	t.Parallel()

	pub := publishedViewWorld("ticket", "feature")
	face := &entity.Entity{
		ID: "FEAT-PUB", Type: "feature", Pointer: entity.Pointer("published"),
	}

	got := pub.provenanceFor(face)
	if got == nil {
		t.Fatal("a non-default world must label its faces — this block is the " +
			"whole reason item 4b exists (RULING 14)")
	}
	if got.Name != "published" {
		t.Errorf("Name = %q, want published", got.Name)
	}
	if got.Pointer != "published" {
		t.Errorf("Pointer = %q, want the coordinate the face was stored at", got.Pointer)
	}
	if got.Via != ruleChain {
		t.Errorf("Via = %q, want %q: `published` is in this world's chain",
			got.Via, ruleChain)
	}

	if defaultViewWorld().provenanceFor(face) != nil {
		t.Error("the DEFAULT world must not label: every entity is its default " +
			"state there by definition, so a block on every row of every " +
			"existing view is noise — and implies a world was applied")
	}
}

// TestViewWorld_ProvenanceDistinguishesFallbackFromChain is the distinction
// the provenance block exists to make, and the one a client cannot re-derive.
//
// A chain-resolved face and a fallback-resolved one arrive BYTE-IDENTICALLY —
// same id, same type, same shape. Only `via` separates "the Dutch page" from
// "the English page, because no Dutch page exists". A provenance block that
// reported the same value for both would be worse than none: it would look
// like an answer.
func TestViewWorld_ProvenanceDistinguishesFallbackFromChain(t *testing.T) {
	t.Parallel()

	// chain [nl], fallback DEFAULT — the multilingual shape.
	w := viewWorld{name: "site-nl", scope: store.NewWorldScope(
		map[string]store.TypeResolution{
			"feature": {
				Chain:    []entity.Pointer{entity.Pointer("nl")},
				Fallback: store.FallbackDefaultState,
			},
		})}

	dutch := w.provenanceFor(&entity.Entity{
		ID: "FEAT-NL", Type: "feature", Pointer: entity.Pointer("nl"),
	})
	fallback := w.provenanceFor(&entity.Entity{
		ID: "FEAT-EN", Type: "feature", Pointer: entity.Pointer(""),
	})

	if dutch.Via != ruleChain {
		t.Errorf("a face from the chain reports %q, want %q", dutch.Via, ruleChain)
	}
	if fallback.Via != ruleFallbackDefault {
		t.Errorf("a face the FALLBACK stood in reports %q, want %q — this is "+
			"the distinction the block exists for, and the one a client cannot "+
			"re-derive without the chain and the fallback policy",
			fallback.Via, ruleFallbackDefault)
	}
	if dutch.Via == fallback.Via {
		t.Error("chain and fallback must not report the same rule")
	}
}

// TestViewEntry_ExcludedEntryIsNotFound pins that a world excluding the ENTRY
// yields a not-found rather than falling back to the default state.
//
// Absence IS the publication bit (§4.1): an entity with no published face does
// not exist in the published world, and serving its draft instead would be the
// exact leak `otherwise: exclude` exists to prevent.
func TestViewEntry_ExcludedEntryIsNotFound(t *testing.T) {
	app := newTestAppV1(t)

	// Draft face only — nothing published.
	seedEntity(app, &entity.Entity{
		ID: "TKT-9", Type: "ticket", Properties: map[string]any{"title": "draft only"},
	})

	if _, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-9", defaultViewWorld()); err != nil {
		t.Fatalf("precondition: the entry must exist in the default world; got %v", err)
	}

	_, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-9",
		publishedViewWorld("ticket", "feature"))
	if err == nil {
		t.Error("an entry the world EXCLUDES must not resolve — serving its " +
			"draft face instead is the leak otherwise:exclude prevents")
	}
}

// TestExecuteView_DefaultWorldUnchanged is the compat assertion.
//
// Every change in item 4b sits behind a default-world branch, and a branch
// written the wrong way round would route ordinary traffic through the world
// path — where a zero WorldScope resolves everything to its default face and
// would LOOK correct while taking a different route with different error
// handling and different batching.
func TestExecuteView_DefaultWorldUnchanged(t *testing.T) {
	app := newTestAppV1(t)
	seedViewGraph(t, app)

	result, err := app.views.executeView(
		context.Background(), implementsView(), "TKT-1", defaultViewWorld())
	if err != nil {
		t.Fatalf("default world: %v", err)
	}
	if got, _ := result.Entry.Properties["title"].(string); got != "draft entry" {
		t.Errorf("the default world serves the default state, as before; got %q", got)
	}
	ids := collectionIDs(result)
	if len(ids) != 2 {
		t.Errorf("the default world collects every neighbor regardless of "+
			"which faces exist; got %v", ids)
	}
	for _, e := range result.Collections["features"] {
		if e.Pointer != entity.Pointer("") {
			t.Errorf("%s: the default world serves default-state rows; got pointer %q",
				e.ID, e.Pointer)
		}
	}
}
